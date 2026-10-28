# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.
Sequencing lives here and in
[`reports/roadmap-sequencing-appdev-security-2026-10-14.md`](reports/roadmap-sequencing-appdev-security-2026-10-14.md)
(the 2026-09-22 R-series stack is superseded); the mission format is
throughline (`skills/throughline/SKILL.md`). Direction:
[`reports/choir-state-and-vision-2026-10-14.md`](reports/choir-state-and-vision-2026-10-14.md).

## Active goal (2026-10-28): data architecture selection

**Goal:** [`definitions/choir-data-architecture-selection-2026-10-28.md`](definitions/choir-data-architecture-selection-2026-10-28.md)
(`readiness: reviewed`, Devin 381 accept). Owner, 2026-10-28: "the data
architecture remains the major bottleneck"; design for many hosts and about
100,000x; memory spikes were the killer. Process: census (memory-capped
planned job), owner budgets and hard gates (bounded peak memory first),
candidates with the incumbent as control, tape-replay bake-off, differential
shadow on d@e, ratify. Working hypothesis in
[`design/data-rearchitecture-2026-10-28.md`](design/data-rearchitecture-2026-10-28.md).

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
[ledger metamission](definitions/choir-ledger-metamission-2026-10-13.md), station LP.

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
patch. Reading: [selfdev update architecture](reports/selfdev-update-architecture-2026-10-15.md),
the [S6 station definition](definitions/choir-appdev-s6-commit-gate-full-release-2026-10-01.md),
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
[desk frame actions are not role-checked by the daemon](problems/desk-frame-actions-not-role-checked-by-host-2026-10-21.md)
and [the jailer copies the shared store image per VM](problems/jailer-copies-store-image-per-vm-2026-10-21.md).
Owner direction: [owner notes §36](reports/owner-notes-2026-10-14.md).

## Overnight run (2026-10-20 → 10-21, owner notes section 32)

**Goal:** [`definitions/choir-overnight-research-oped-sources-2026-10-20.md`](definitions/choir-overnight-research-oped-sources-2026-10-20.md)
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
[`definitions/choir-tape-segments-a2-repack-2026-10-21.md`](definitions/choir-tape-segments-a2-repack-2026-10-21.md)
(round 5 and r1 landed; owner notes 34: the write flag is on for every
computer, repack next, legacy deletes gated on evidence and the owner's
yes; see its now card). The delegate's one-round build landed
(design/delegate-one-round-build-2026-10-21.md); the Sonnet rewrites
resume with SlidesApp.

## Foundations push (2026-10-14 evening → 10-15 15:00; owner: deepen foundations)

Live today: LF0/LF1 front page, S2a+S2b management tasks, artifact GC,
search benching fix + cache + free-first routing, Jev peer re-pin (L2a
labels flowing), headline titles, snapshot FIEMAP measure, 28 sources;
deploying: object-graph write gate, dead kind, matched quote spans.

Landing order (one deploy at a time, each verified on staging before the
next; branches built in parallel, landed in dependency order):
1. tap firewall (fix/tap-deny-guest-range) + established-only inbound;
2. internal-route authentication in observe mode, Dolt password, unit
   hardening (then enforce);
3. guest store indexes (CPU C1-C3, C5) + kvm.halt_poll_ns=0;
4. Firecracker jailer slice 1 (flag off → one test computer → all);
5. admission by measured use + sustained-pressure reclaim;
6. desk cells confined + guest egress policy;
7. prompt-cache stability;
8. Store B retrieval (timing, indexes, SQL corpus search);
9. account recovery (LA slice 1);
10. VM backend interface + Cloud Hypervisor slice — design now, code after
    4 and 5 land (same files).
Status 2026-10-21 (see the checklist above): 1 half done (F7 landed; the
established-only inbound rule is not built); 2 enforces, Dolt auth at
stage "users"; 4 landed and off, rollout authorized, and the jail copies
the store image per VM; 5 landed; 6 not landed, its rebased branch is now on
origin as `land/desk-cell-confinement` (recovered from the owner's iMac
2026-10-21); 9 landed; 10 not on main, its seam is on origin as
`recovered/vmbackend-2026-10-14`.
Alongside, no deploy: self-development drill rerun, claims E0 + E6a,
Autoradio prototype (branch), teaching series (docs/learn/).
Owner decisions list: delivered as the 2026-10-20 digest and answered; the
rulings are [owner notes §27](reports/owner-notes-2026-10-14.md). Open items
are the account-recovery launch blocker (§27 item 13). (The Telegram
MTProto setup was closed later: the owner skipped Telegram, owner notes
"Telegram skipped".)

## Now (2026-10-14, 07:00 UTC): what landed overnight, what is next

**Deployed tonight** (staging 9ace75e5; every deploy kept the site up,
health probes 200 throughout):
- C1e B1, engineering on records (e9d727a7), with the logged-out home
  opening straight to Autopaper and London Salmon as the default theme.
  The capsule-gate fix (bc09dcc1) and the Freeze fixes (9874ebc5,
  96b82cec) followed; the engineering drill still cannot reach Freeze
  because capsules cannot fetch Go modules
  ([decision for the owner](reports/capsule-go-modules-decision-2026-10-14.md)).
- LR: lever 6 (cell output out of the transcript), lever 7 (prices;
  `/api/costs` reports dollars), LongCat out of the default pool (too
  slow: [evidence](evidence/model-latency-paper-2026-10-14.md)), Jev
  judges citation support off management's turn, the research-prompt
  renderer and prompt fixes, the balloon (free page reporting) and soft
  Go memory limits.
- S3 real hibernation: a memory snapshot instead of a kill; wake to
  healthy in 103 ms against 9.7 s cold, open work kept
  ([evidence](evidence/s3-snapshot-hibernate-staging-2026-10-14.md)); the
  credential window (resume past the guest capability's expiry, up to
  12 h) deployed in 9ace75e5, proof in progress.
- L2a fixture passes (merge recall 1.0, 0 false merges); next is the
  paper's enable.

**Owner priority change (2026-10-14, about 12:00 UTC):** "I actually want
to accelerate doing the optimizations. [...] even the auto paper scaling
should come after these optimizations because we really shouldn't be
scaling our utilization until we improve our efficiency to the point of
diminishing marginal returns." Optimizations now come first: the artifact
store leak (docs/problems/artifact-store-grows-12gb-a-day-2026-10-14.md),
snapshot budget by real bytes, cached-token measurement and prompt-cache
hits, guest memory right-sizing, the daemon's store memory, KSM (to be turned
off, not used: checklist SEC-09), disk cleanup; then Autopaper scaling. "Always on" is renamed **available**
(running, or snapshot-hibernated and waking in under a second).

**Owner priorities (earlier 2026-10-14):** Autopaper scaling first; the data
model (object graph on Dolt) always; app packages (station LP: S6 then S7,
existing apps converted first in S7-0) before building any new app; guest
updates (station LU) and the capsule Go module proxy alongside. Puter
review for app agents: `docs/research/puter-everything-2026-10-14/README.md`.
Autopaper front-page ranking and personalization (station LF) and reader
contributions, op-eds and editing (station LC), intent only, design first:
`docs/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md`.
Sequencing proposal for the rest of the app-development metamission,
auth (station LA: recovery is broken today) and security (SP before
agentic red teaming, station LT):
`docs/reports/roadmap-sequencing-appdev-security-2026-10-14.md`.

**Waiting on the owner:** the Go-module route for capsules (A, B or C);
SP0's host, recovery targets, and whether the escrow custodian key and
platform signing seed have an offline copy; the 12 h snapshot maximum
age; the reader spec decision.

**Next:** enable intake.labels on the paper after an idle refresh and
measure against baseline A; management delegating to sub-RLMs (owner
direction; design first); the balloon's return measured after desk
workers exit; the daemon's store memory (the real density lever:
[split](evidence/guest-memory-split-balloon-2026-10-14.md)); research
searching the corpus first. Residual for 10-14/15: Autopaper across
several computers on one host
([analysis](reports/cost-track-sa-s3-sp-investigation-2026-10-14.md)).

**New station LU, guest updates** (owner-approved 2026-10-14; drafted):
[station file](definitions/choir-ledger-lu-guest-updates-2026-10-14.md),
[design note](reports/lu-guest-updates-design-2026-10-14.md),
[panel adjudication](reports/lu-guest-updates-adjudication-2026-10-14.md).
Pull with a bounded drain instead of a once-per-deploy push, so the busy
paper updates by itself; planned boots across epoch changes; rings,
health and rollback; one apply path with self-development. Reviewed per
slice (round 4): LU1a and LU2 may start; LU3a, LU3b, LU4 reviewed; LU1b
(the drain's final call) escalated for a structural decision after four
rounds; LU5 needs its own design round.
LU2 must land before C1e's next epoch bump.

## Next (2026-10-13): ledger metamission (reviewed)

The [ledger metamission](definitions/choir-ledger-metamission-2026-10-13.md)
turns the roadmap the owner asked for into stations: L0 closes the
re-engineering metamission below (E3 rerun passed, A1 badge, C1e, final
review); L1 ends crash loops
([station file](definitions/choir-ledger-l1-restart-2026-10-13.md) over the
[restart design v3](restart-subsystem-design-2026-10-13.md); L1a-1 and
L1a-2 landed, L1a-3 next, before C1e); L2 bounded
attention; L3 hypotheses settled and scored; L4 Taiwan and East Asia as
standing questions; L5 the evidence reader and an Autoradio prototype. It
stays open to additions. No owner decisions are open on the spine: the
cost amendment, Telegram and commercial status, Taiwan first, Autoradio's
slot and schedules option B were answered 2026-10-14
([rulings](reports/owner-roadmap-rulings-2026-10-14.md)), and the
2026-10-20 decisions digest was answered in
[owner notes §27](reports/owner-notes-2026-10-14.md). Convergent round 1 sent the spine back; round 2
accepted the repaired spine and the L1 station file (2026-10-13). L1a-3
(crash boots settle from the updater journal, never re-apply) landed at
18d01cc0. Cross-model cell errors (owner 2026-10-13: errors every model
makes point at the tool and prompt surface): report at
docs/reports/cross-model-cell-errors-2026-10-13.md; fixes for clusters
A (stopgap), B, D, E, F, G, the hint scanner and the Texture and
management prompts pushed at d2324a55; cluster C's decode hint at
cdffb40d (unpushed until the paper run); the typed Read result and the
yaegi substrate fix for cluster A are residuals. The paper tracer's blind
spot is fixed at 29bbab97 (owner-scoped publication read). The paper run
on d2324a55 measured the cell-error change (37/283 failing cells against
165/828; clusters A-G down to 6 cells). C1e
(docs/definitions/choir-reeng-c1e-engineering-on-records-2026-10-13.md)
is reviewed and B0 is landed at 19e56c54: the runtime census of work
items and packets, I7 read from it, I2 and I3 on records, the checker's
--require for missing evidence, and the tracer off work_items. The paper
run passed and the deployed checker read I7 pass. The step-7 uncertainty,
answered on the code read
(docs/reports/c1e-management-packet-reads-2026-10-13.md), moves into B1:
the Report lacks the assignment identity and a typed outcome, and the
records wake cannot resume a parked caster. The B1 design note
(docs/reports/c1e-b1-design-note-2026-10-13.md) is in convergent round 3
after two send_backs.

2026-10-14: the paper computer was hibernated under host pressure at
21:00 UTC. The owner ruled computer tiers (paid always on; Autopaper a
platform tier above every user tier), landed at 188f98c3; Autopaper is
live again since 23:53 UTC. The owner then asked for the resource and
cost mission tonight: station LR
([file](definitions/choir-ledger-lr-resource-2026-10-13.md), over the
[assessment](reports/resource-efficiency-memory-assessment-2026-10-13.md)).
Lever 1 (signal-specific reclaim) is live at d3f7d9b2; lever 2 (test
tier) and L1's L1b-0 (the always-on sweep warms a failed computer once
per 24 h) are pushed at 6c6bc1aa. Next: verify them, adjudicate B1 round
3, then the lever-6 design (cell output out of the transcript).

## Current (2026-10-13): re-engineering metamission

The live spine is the
[re-engineering metamission](definitions/choir-reeng-metamission-2026-10-12.md):
three cuts to Autopaper live, with decisions in
[re-engineering-decisions-2026-10-12.md](re-engineering-decisions-2026-10-12.md).

- **M0, model pools: landed.** Code b0d69d4d; staging acceptance passed
  ([design and result](model-pools-design-2026-10-12.md)). Every desk runs
  on deepseek-v4.1-flash and gpt-6-luna, each the other's fallback.
- **D0, dead-code census and removal: landed.** About 50,000 lines
  deleted, all owner-approved
  ([census and rulings](reports/dead-code-census-2026-10-12.md)).
- **C0, eyes: landed 2026-10-13.** Trace export without SSH, a tracer
  with a crash drill, an invariant checker, request-id routing, one REPL
  per desk, broker handle reissue
  ([receipts](evidence/c0-eyes-acceptance-2026-10-13.json),
  [measurements](reports/reeng-cut0-measurements-2026-10-12.md)).
- **C1, authority: core landed 2026-10-13 (76ab40fd).** Crash closure;
  records as the work state for Texture with research and management;
  prompts that teach only callable verbs
  ([cut file](definitions/choir-reeng-cut1-authority-2026-10-12.md),
  [crash](evidence/c1-core-crash-acceptance-2026-10-13.json) and
  [management](evidence/c1-core-mgmt-acceptance-2026-10-13.json) receipts).
  C1e (engineering on records, wholesale deletions) runs after C2's live
  window.
- **C2, evidence to publication: Autopaper live 2026-10-13.** The
  designated paper computer publishes in public from its schedules
  ([receipt](evidence/c2-autopaper-live-2026-10-13.json)); stories open in
  the Texture app. Residual items in the cut's now card. Next: C1e, the
  owner computer's update path, then the completion review.
- **C2 pre-flight (done).** Pre-flight
  ([drift](reports/reeng-cut2-preflight-2026-10-13.md),
  [quote census](reports/reeng-c8-quote-census-2026-10-13.md)); unit 2a
  next, ending with Autopaper live.
- **Pace (owner 2026-10-13):** fast loop, no long runs; one
  claim-directed tracer scenario per landing (metamission
  `boundaries.pace`).

The roadmap below is the 2026-10-09 gate view; the cuts replace its Gate 3
plan.

## Roadmap (owner-ratified 2026-10-09)

Three gates, in order, each with an exit test — see "v6 plan" in the
[metamission](definitions/choir-supervised-app-development-metamission-2026-10-01.md):

1. **Stable computer** — SH (O21) → SL (O1) → O7/O12 checks → O8/O9 →
   Texture contract. Exit: lose-the-disk proof, SL fault matrix, Texture
   acceptance, and owner confidence in the autoputer. Stable uptime is
   counted, not waited for (owner 2026-10-09: no 72 h clock).
2. **Self-development with live Texture supervision** — S1 → S4 → S5 → S6.
   Exit: owner-requested change, capsule build, live preview, approval,
   release, rollback.
3. **Autopaper** (formerly the World Wire; owner 2026-10-09: Gate 1 today,
   Gate 2 tomorrow, Gate 3 in October; design draft
   [`autopaper-sw-design-2026-10-09`](autopaper-sw-design-2026-10-09.md);
   name retirement is a TODO there, §9) — four core desks, minimal
   structure, living Textures woken by evidence, automatic publishing into
   the host-level store, starting from zero content.

Deferred: S3, SA density, S7–S11, and
[SP production infrastructure](definitions/choir-appdev-sp-production-infrastructure-2026-10-09.md)
(O22; until Gates 1–2 pass human QA). Corpus (Store B) teardown owner-approved
2026-10-09; processor/reconciler deletion committed.

## Persistent desks metamission (2026-10-11; drafted successors)

[`definitions/choir-persistent-desks-metamission-2026-10-11.md`](definitions/choir-persistent-desks-metamission-2026-10-11.md)
(`readiness: executable`): one persistent research desk per document (PD1,
[station file](definitions/choir-pd1-persistent-research-desk-2026-10-11.md))
is the worked example. Successor stations are drafted, not scheduled:
[PD2 sessions outlive runs](definitions/choir-pd2-desk-sessions-outlive-runs-2026-10-11.md),
[PD3 Texture root desk](definitions/choir-pd3-texture-root-desk-2026-10-11.md),
[PD4 computer controls computer](definitions/choir-pd4-computer-controls-computer-2026-10-11.md).
Re-engineering (above) superseded parts of their sequencing; check its
decisions before starting any of them.

## Current Mission — Supervised App Development Metamission

[`definitions/choir-supervised-app-development-metamission-2026-10-01.md`](definitions/choir-supervised-app-development-metamission-2026-10-01.md)
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
  [`problems/sa-projection-base-watermark-never-refreshed-2026-10-07.md`](problems/sa-projection-base-watermark-never-refreshed-2026-10-07.md).
- **SL** obligation terminality **working** (opened 2026-10-08, owner
  direction "proceed with mission next"):
  [`definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md`](definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md)
  — enforce operational invariant O1 from the
  [operational invariants register](operational-invariants-register-2026-10-08.md)
  (draft, owner-approved set). Absorbs SA slice 1 (Management storm
  convergence). First slice: read-only obligation inventory.
- **SH** state homes **working** (opened 2026-10-08, owner direction
  "good policy and good ideas. Let's do it"):
  [`definitions/choir-appdev-sh-state-homes-2026-10-08.md`](definitions/choir-appdev-sh-state-homes-2026-10-08.md)
  — enforce O21, a realization holds no unique state
  ([inventory](state-homes-inventory-2026-10-08.md)). Escrowed privacy key
  delivered to a computer's new realization (owner-ratified). Slice 1: typed
  `privacy_key_unavailable` refusal and prompt guest fatal-startup reporting.
Ordered ahead of the Autopaper work per owner direction 2026-10-01
("a good prerequisite to it").

## Parallel Mission — Jev / Supervision Metamission

[`definitions/choir-jev-supervision-metamission-2026-09-29.md`](definitions/choir-jev-supervision-metamission-2026-09-29.md)
is still `working` but no longer the live entrypoint — the app-dev
metamission took spine position 2026-10-01 on owner approval. Its
station files and receipts remain valid; it resumes as parallel work or
is folded when its remaining stations intersect S1-S11.
## Parallel Mission — Texture Latency Cutover

[`definitions/choir-texture-latency-cutover-2026-10-01.md`](definitions/choir-texture-latency-cutover-2026-10-01.md)
(`readiness: executable`, `now.status: working`) — owner-directed
2026-10-01: bound Texture editor request latency end-to-end (proxy resolve +
guest store + lifecycle snapshot). Three ordered cuts: trajectory-scoped or
version-memoized snapshot read, TTL-cached vmctl route resolution, and —
only if the timing matrix still fails — decoupling the objectgraph read
mutex from writes. Evidence map:
[`problems/texture-latency-layered-evidence-2026-10-01.md`](problems/texture-latency-layered-evidence-2026-10-01.md).
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
and [`reports/roadmap-sequencing-appdev-security-2026-10-14.md`](reports/roadmap-sequencing-appdev-security-2026-10-14.md).

## Graph Rule

[`mission-graph.yaml`](mission-graph.yaml) is discovery metadata; Definitions
own state, not Git history.
