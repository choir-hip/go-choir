# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.
Sequencing lives here and in
[`reports/roadmap-sequencing-appdev-security-2026-10-14.md`](archive/reports/roadmap-sequencing-appdev-security-2026-10-14.md)
(the 2026-09-22 R-series stack is superseded); the mission format is
throughline (`skills/throughline/SKILL.md`). Direction:
[`reports/choir-state-and-vision-2026-10-14.md`](archive/reports/choir-state-and-vision-2026-10-14.md).

## Active goal (2026-10-30): the lean system

**Goal:** [`definitions/choir-lean-system-2026-10-30.md`](definitions/choir-lean-system-2026-10-30.md)
(`readiness: executable` on the owner's word). Run it with
`/goal docs/definitions/choir-lean-system-2026-10-30.md`.

Owner, 2026-10-30: "let's have another aggressive night." Stations: L0 free
the disks on Node A and Node B; L1 the paper an order of magnitude smaller;
L2 a crashed computer comes back, a crash loop does not; L3 the docs
describe the running system; L4 the data system's phase 2 (I1 to I6); L5
the off-host backup, streamlined (I7); L6 loose ends and the handoff.

## Superseded (2026-10-30): the data system build

**Goal:** [`definitions/choir-data-system-build-2026-10-30.md`](definitions/choir-data-system-build-2026-10-30.md).
Phase 1 landed on 2026-10-30: the fleet runs on PostgreSQL on the host and
SQLite in the computers; Store A, Store B and the guest Dolt workspaces are
deleted. Its phase 2 carries over to the lean system as L4 and L5. The frame,
[`design/choir-data-system-frame-2026-10-30.md`](design/choir-data-system-frame-2026-10-30.md),
stays the design source.

## Superseded (2026-10-30): data architecture selection


**Goal:** [`definitions/choir-data-architecture-selection-2026-10-28.md`](definitions/choir-data-architecture-selection-2026-10-28.md)
(`readiness: reviewed`, Devin 381 accept). Owner, 2026-10-28: "the data
architecture remains the major bottleneck"; design for many hosts and about
100,000x; memory spikes were the killer. Process: census (memory-capped
planned job), owner budgets and hard gates (bounded peak memory first),
candidates with the incumbent as control, tape-replay bake-off, differential
shadow on d@e, ratify. Working hypothesis in
[`design/data-rearchitecture-2026-10-28.md`](archive/design/data-rearchitecture-2026-10-28.md).

## Superseded (2026-10-28): consolidate and scale

[`definitions/choir-consolidate-and-scale-metamission-2026-10-26.md`](definitions/choir-consolidate-and-scale-metamission-2026-10-26.md)
dropped by the owner; its terminal receipt lists what landed and what
carries forward.

## Settled (2026-10-26): self-development readiness, checkpoint_incomplete

Was: Active metamission (2026-10-24): self-development readiness

**Goal:** [`definitions/choir-selfdev-readiness-metamission-2026-10-24.md`](definitions/choir-selfdev-readiness-metamission-2026-10-24.md)
(`readiness: executable` on the owner's word; file review pending). Owner,
2026-10-24: "self-development is our priority"; "do all of the platform
upgrades that relate to self-development now"; "supersede our current one
and start the new one". Acceptance: the owner's loop run end to end on the
Reader app (a coding agent asks a test computer through the choir CLI,
engineering builds on current main, the owner approves, the change is
promoted to main and reaches every computer as the base). Stations: SR1 the
source follows the base (U1, U1b); SR2 app packages
([replan](design/app-packages-replan-2026-10-24.md)); SR3 the loop's gaps;
SR4 the one-home migration (doctrine I27); SR5 the Reader loop; SR6 the
first pass's carried items. Then the tent poles: app agents, an agentic
red-teaming loop, distribution. Cadence: switched-off slices share a deploy
wave; each is turned on for one test computer at a time.

**Now (2026-10-26 00:5x):** PD3 is complete (the Texture root desk and
master document, S1 and S2, live on b@c). The live station is
[T1a, the apps to a design bar](definitions/choir-t1a-app-self-development-2026-10-26.md):
the Reader redesign awaits approval on b@c, then Slides, Files (Open with),
Calendar and Podcasts, with Synthetic DeepSeek and the Claude delegate
(live on b@c). SR9, propagating accepted changes by an internal mechanism,
is in investigation. Then T1c (the desktop redesign).

## Superseded (2026-10-24): the launch first pass

[`definitions/choir-launch-first-pass-metamission-2026-10-21.md`](definitions/choir-launch-first-pass-metamission-2026-10-21.md)
landed and proved deploys through D44 and closed as superseded (terminal
receipt in the file). Its open items continue as station SR6 above.

## Active station (2026-10-20): LP S6, the commit gate

**Goal:** [`definitions/choir-lp-s6-commit-gate-2026-10-20.md`](definitions/choir-lp-s6-commit-gate-2026-10-20.md)
(`/goal` file, `readiness: reviewed`, verdict accept with edits). Under the
[ledger metamission](archive/definitions/choir-ledger-metamission-2026-10-13.md), station LP.

**Priority change (owner, 2026-10-20, [owner notes §28](reports/owner-notes-2026-10-14.md)):**
"yes, we should do the missions in order through app packages as recommended."
Order: S6 (what runs is what was approved), then S7-0 (every first-party app
becomes a package) with a frontend self-development drill on one rewrite
candidate in parallel, then rewrites of the weaker apps (not mail, settings,
autopaper or texture), one package at a time. This moves LP ahead of the
2026-10-14 order (efficiency, Autopaper scaling, then LP) and relaxes LP's edge
to L2a, LR and LU: S6 needs none of them. The foundations list and the
Autopaper work below continue alongside; nothing below is superseded.

**First slice:** S6-0, a release witness that joins decision, bundle, test
result, host-build digests, applied release and heads, and names each join as
compared at the gate or only recorded; proven by the go-served tracer drill on
staging. **Open for the owner (defaults in force):** whether a failing declared
test refuses the owner's own approval; whether to approve the closure or the
patch. Reading: [selfdev update architecture](archive/reports/selfdev-update-architecture-2026-10-15.md),
the [S6 station definition](archive/definitions/choir-appdev-s6-commit-gate-full-release-2026-10-01.md),
and the S7 definition for what follows.

## Daytime work (2026-10-21, owner notes section 35)

The owner raised the priority of dual-model tool-call evaluation above the
SlidesApp rewrite, and of desk-wake delay: "the delays for waking a desk.
There should be no delays." Records:
[`problems/research-more-slow-and-hidden-2026-10-21.md`](problems/research-more-slow-and-hidden-2026-10-21.md)
(E6: the dispatcher's slots full and served in agent-id order, over a
store engine 96% busy; fix order inside) and
[`problems/tool-call-failures-dual-model-2026-10-21.md`](problems/tool-call-failures-dual-model-2026-10-21.md)
(T1 to T5, the census by model). Landed: Research more goes straight to
research, the toolbar's overflow menu, the deepseek-v4.1-flash and
gpt-6-luna pool (1 to 3), the dispatcher's oldest-due-first order, and
three store reads moved off scans. Next: measure the paper after each
deploy, then the remaining hot reads, then engine concurrency.

## Launch checklist (2026-10-21; first draft for owner review)

[`launch-checklist-2026-10-21.md`](launch-checklist-2026-10-21.md) is the
outer checklist: every launch feature and requirement in the intended order,
from the Apple developer account to the Autoradio app, by way of the voice
pipeline, self-development, accounts and the App Store submission (target
2026-10-31). Its phase 2 points at the security checklist below without
copying it. Revised the same afternoon after a three-model panel
(GPT-6.1-sol, Gemini 4 Argon, Devin): its batch plan is now the one
ordered list for both checklists, and its owner decisions OD-01 to OD-09
carry the YouTube research (Premium cannot be promised inside an embed).

## Security, density and architecture checklist (2026-10-21; owner: as soon as possible)

[`security-density-architecture-checklist-2026-10-21.md`](security-density-architecture-checklist-2026-10-21.md)
is the tick list for desk isolation, guest egress, memory and the app-desk
and scale-out architecture: each item says what is on main, on branches and
in plans, with **P0** items before launch and a suggested P0 order. It
found that the desk-cell confinement branch (landing order item 6 below)
existed only on the owner's iMac (pushed 2026-10-21 as
`land/desk-cell-confinement`), and two new problems:
[desk frame actions are not role-checked by the daemon](problems/desk-boundary-residuals-2026-10-21.md)
and [the jailer copies the shared store image per VM](problems/jailer-copies-store-image-per-vm-2026-10-21.md).
Owner direction: [owner notes §36](reports/owner-notes-2026-10-14.md).

## Overnight run (2026-10-20 → 10-21, owner notes section 32)

**Goal:** [`definitions/choir-overnight-research-oped-sources-2026-10-20.md`](archive/definitions/choir-overnight-research-oped-sources-2026-10-20.md)
(`readiness: drafted`). After the backup snapshot path lands: Research more
for every reader (logged out included), op-eds as connected Texture
publishing with an Opinion section, wave-1 schedules plus AT Protocol and
Fediverse leads, then the failure 15 carried patch and a two-arm frontend
self-development drill (Sonnet through the Claude delegate versus GPT-6
Luna) on test computers. S6 above stays the active LP station.

Status at 05:30 UTC on 2026-10-21: the backup snapshot path, Research
more (logged out), op-eds and the wave-1 and social rows (Taiwan included)
are deployed and proven; the two-arm drill ran (report:
[`reports/selfdev-two-arm-drill-imageapp-2026-10-21.md`](reports/selfdev-two-arm-drill-imageapp-2026-10-21.md)).
Owner notes section 33 changed the rest of the night: no more Luna arms;
Sonnet rewrites of Slides, Pdf, Epub, Podcast, Calendar and FileBrowser, one
at a time; and a durable fix for the backup verify through the tape segment
store, goal
[`definitions/choir-tape-segments-a2-repack-2026-10-21.md`](archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md)
(round 5 and r1 landed; owner notes 34: the write flag is on for every
computer, repack next, legacy deletes gated on evidence and the owner's
yes; see its now card). The delegate's one-round build landed
(design/delegate-one-round-build-2026-10-21.md); the Sonnet rewrites
resume with SlidesApp.

## Earlier sections pruned (2026-10-30)

The foundations push (10-14 to 10-15), the 10-14 overnight status, the ledger
metamission, the re-engineering metamission and the 10-09 roadmap sections were
removed: they presented Dolt and Store B as live, and both were deleted in the
2026-10-30 data system cutover. They remain in the repo's version history.

## Persistent desks metamission (2026-10-11; drafted successors)

[`definitions/choir-persistent-desks-metamission-2026-10-11.md`](archive/definitions/choir-persistent-desks-metamission-2026-10-11.md)
(`readiness: executable`): one persistent research desk per document (PD1,
[station file](archive/definitions/choir-pd1-persistent-research-desk-2026-10-11.md))
is the worked example. Successor stations are drafted, not scheduled:
[PD2 sessions outlive runs](archive/definitions/choir-pd2-desk-sessions-outlive-runs-2026-10-11.md),
[PD3 Texture root desk](archive/definitions/choir-pd3-texture-root-desk-2026-10-11.md),
[PD4 computer controls computer](archive/definitions/choir-pd4-computer-controls-computer-2026-10-11.md).
Re-engineering (above) superseded parts of their sequencing; check its
decisions before starting any of them.

## Current Mission — Supervised App Development Metamission

[`definitions/choir-supervised-app-development-metamission-2026-10-01.md`](archive/definitions/choir-supervised-app-development-metamission-2026-10-01.md)
(`readiness: executable`, `now.status: working`) — **owner-approved
2026-10-01** after agentic-consensus review (5/5 convergent + 7/7
divergent; all required revisions applied in v3). Thirteen stations;
2026-10-04 three closed in one pass:

- **S0m** record-native messaging **complete** (boundary
  `s0m-to-next-transition-2026-10-04`; ask→report→resolve
  deployed-verified on trajectory `684ddcb1`; consume-marking `d61c9b1b`;
  reporter letter `reports/s0m-record-native-station-close-2026-10-04.md`).
- **S1a** host-boundary hotfix **complete** (boundary
  `s1a-to-s0b-transition-2026-10-04`; deployed refusal matrix PASSED —
  5/5 refusals incl. tap→tap FORWARD drop + forged-identity 403/405,
  5/5 legitimate flows green; reporter letter
  `reports/s1a-host-boundary-station-close-2026-10-04.md`).
- **S0** reality + boot timeline **complete** (boundary
  `s0b-boundary-close-2026-10-04` on the S0 station file; 10 probe
  receipts on disposable `computer-ac1808b4` + second disposable for the
  Go-effect re-run; two new problem docs: M9a route-projection
  owner-binding defect + selfdev ops-launch freeze-authority gap;
  builder-substrate decision narrowed to host-service vs
  privileged-builder-capsule).

- **S2** layering-runtime-from-release **CLOSED** (2026-10-05, terminal
  receipt `s2-station-terminal-2026-10-05`): all six acceptance criteria
  discharged at deployed-proof class on disposable `computer-6450a253`
  plus the owner computer — builder-produced no-reboot apply with
  built-bytes frontend join, CI-driven M9a push (run 37378327268,
  2/2 healthy, t2h=131s), six refusal fences, post-disposal+reboot
  base-absent refusal, GC-rooted private-store boundary, clean rollback.
  Consensus round 2: 6 approve / 1 send-back (sole send-back receipt
  completion only, discharged by fc/boot continuity pins). Contract
  slices landed: S2-e rollback atomicity, S2-d state-compat gate, S2-c
  provenance, S2-f builder join, S2-g CI wiring. Open residuals (problem
  docs, not blockers): post-swap guest restart loop, canonical-head
  bootstrap authority, platform-artifacts GC cadence (S0 — first sweep
  reclaimed 35.4GB on Node B). Next spine station per v5: S3 fast resume
  after SR/SA density work; parallel: S1 remainder + SO ops substrate.
- **SMG** management RLM cutover **CLOSED** (2026-10-06, spine receipt
  `smg-to-sa-transition-2026-10-06`): legs 1-3 passed on a fresh
  disposable (computer-0ca7656f, build 475902d7,
  `docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json`) via the
  deterministic `POST /api/texture/management-open` acceptance surface;
  deployed schema leg = management exactly `{desk_go_eval}`
  (computer-03335285). Named edge: re-run legs on the owner computer
  after SA slice 1 (incl. weak legs: cell-level unbound refusal +
  work settlement). New substrate defects handed to SA slice 1:
  delegated-report poisoned delivered-page listing + mint-no-start slot
  deadlock (fixed 01199fb1) + AppendEvent unbounded scan (fixed
  475902d7) + wake-outbox re-arm storm.
- **SA** agent density **working** (slice 0 deployed 9f6f369c; slice 0
  residuals landed 2026-10-07: delegated-report delivered-page fix
  `ca8c8c18` + apply-fence liveness-tolerant probe `19d7913e` +
  commit-bound push gate `68397ae7`; owner runs `19d7913e`. Slice 1
  storm convergence open; drain fixes landed `91e9c03b`/`29817fda`/
  `5eb63161`/`7a36713c`. Management-open wake race fixed `e9cd9fed`
  (problem doc `smg-management-open-invalid-transition-2026-10-07.md`);
  deployed-verified 2026-10-07 on fresh disposable `computer-e472d237`
  (legs 1-3 + bound report green, evidence
  `smg-rlm-acceptance-fix-verify-2026-10-07.json`).
  Owner-VM boot-loop incident (2026-10-07) → permanent recovery policy
  landed and deployed 2026-10-08 (`67729e07`): verified incremental
  checkpoints (`checkpointd`), typed vmctl admission, deposit upcast.
  Cadence/replay repaired-with-residual; admission awaits its staging
  refusal demo. Record:
  [`problems/sa-projection-base-watermark-never-refreshed-2026-10-07.md`](archive/problems/sa-projection-base-watermark-never-refreshed-2026-10-07.md).
- **SL** obligation terminality **working** (opened 2026-10-08, owner
  direction "proceed with mission next"):
  [`definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md`](archive/definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md)
  — enforce operational invariant O1 from the
  [operational invariants register](operational-invariants-register-2026-10-08.md)
  (draft, owner-approved set). Absorbs SA slice 1 (Management storm
  convergence). First slice: read-only obligation inventory.
- **SH** state homes **working** (opened 2026-10-08, owner direction
  "good policy and good ideas. Let's do it"):
  [`definitions/choir-appdev-sh-state-homes-2026-10-08.md`](archive/definitions/choir-appdev-sh-state-homes-2026-10-08.md)
  — enforce O21, a realization holds no unique state
  ([inventory](operational-invariants-register-2026-10-08.md)). Escrowed privacy key
  delivered to a computer's new realization (owner-ratified). Slice 1: typed
  `privacy_key_unavailable` refusal and prompt guest fatal-startup reporting.
Ordered ahead of the Autopaper work per owner direction 2026-10-01
("a good prerequisite to it").

## Parallel Mission — Jev / Supervision Metamission

[`definitions/choir-jev-supervision-metamission-2026-09-29.md`](archive/definitions/choir-jev-supervision-metamission-2026-09-29.md)
is still `working` but no longer the live entrypoint — the app-dev
metamission took spine position 2026-10-01 on owner approval. Its
station files and receipts remain valid; it resumes as parallel work or
is folded when its remaining stations intersect S1-S11.
## Parallel Mission — Texture Latency Cutover

[`definitions/choir-texture-latency-cutover-2026-10-01.md`](archive/definitions/choir-texture-latency-cutover-2026-10-01.md)
(`readiness: executable`, `now.status: working`) — owner-directed
2026-10-01: bound Texture editor request latency end-to-end (proxy resolve +
guest store + lifecycle snapshot). Three ordered cuts: trajectory-scoped or
version-memoized snapshot read, TTL-cached vmctl route resolution, and —
only if the timing matrix still fails — decoupling the objectgraph read
mutex from writes. Evidence map:
`problems/texture-latency-layered-evidence-2026-10-01.md`.
Independent of the capacity mission's substrate work; runs parallel.

## History (pruned 2026-10-14)

Older sections of this view (Gate 1 hardening receipts, the superseded
retired World Wire metamission, the RLM cutover and substrate "completed
definition" blocks, archived R-series receipts, draft successors) were
removed in the 2026-10-14 docs prune; they lived as status diary, not
authority. Landed station definitions are under
[`archive/definitions/`](archive/definitions/); the other removed material is
listed with reasons in [`archive/PRUNED-2026-10-14.md`](archive/PRUNED-2026-10-14.md)
and recoverable from Git history. Recorded problems are in
[`problems/`](problems/), evidence receipts in [`evidence/`](evidence/).
The superseded 2026-09-22 R-series roadmap is
[`world-wire-mission-stack-2026-09-22.md`](world-wire-mission-stack-2026-09-22.md)
(kept only because AGENTS.md names it); sequencing now lives in this file
and [`reports/roadmap-sequencing-appdev-security-2026-10-14.md`](archive/reports/roadmap-sequencing-appdev-security-2026-10-14.md).

## Graph Rule

[`mission-graph.yaml`](mission-graph.yaml) is discovery metadata; Definitions
own state, not Git history.
