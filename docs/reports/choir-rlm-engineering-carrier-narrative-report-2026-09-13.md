# Mission 3: RLM Engineering Carrier — Narrative Report

**Date:** 2026-09-13 (written at mission HEAD `3680c125`; mission in flight)
**Definition:** `docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md`
**Status:** P0–P4 implemented and locally audited, with the per-operation replay proof deployed; the P5 mechanism is proven up to the terminal commit with the roster receipt quarantined and the P5 gate open; P6 landing not started
**Duration:** ~49 hours so far (charter 2026-09-11 18:09Z → HEAD 2026-09-13 18:43Z)
**Mutation class:** red (protected surfaces: gateway/provider calls and model routing, assignment-fate saga and its lifecycle CAS, run acceptance, canonical events and evidence projection, capsule execution and the capability broker; Texture canonical writes as a design constraint)

---

## 1. Summary

Mission three moves the engineering desk entirely off the JSON tool tray and onto
the in-cell carrier: one model-independent prompt, one REPL cell surface
(`capsule_go_eval` plus typed `choir.*` functions), one reducer that authors the
assignment fate, and an acceptance layer that derives from canonical evidence
instead of tool names. The five overlay JSON tool names were not hidden — they
were deleted from the desk's registry (the one desk-scoped exception,
`update_coagent`, survives for the Texture and research desks and is asserted
absent from the engineering carrier by test), and each deletion was earned by a
per-operation replay proof that demonstrates the in-cell successor returns the
original receipt.

By HEAD the carrier build is implemented and locally audited, with the
per-operation replay proof deployed: the simplified envelope, reducer-owned
settlement, the in-cell freeze/verify/inspect surface, the singleton assigned
registry, the five deletions, and the replay harness with golden receipts. The
proving ground is the roster — a frozen desk task served to a real model
through the new carrier on the retained staging computer. That is where the
mission now lives: the mechanism chain is proven up to the terminal commit (the
first arm ever served by its own funded overlay; the desk task's three cells
executed inside the capsule on the served objective), but the terminal receipt
is quarantined because the reducer's terminal saga stranded at
`revoke_requested` — a member of a six-strand defect family in the lifecycle
substrate — and the escalation to the owner is the live next action.

## 2. What the mission set out to do

The charter (ratified by the owner 2026-09-11) names the product outcome: *the
engineering desk lives entirely on the in-cell carrier and nothing else.*
Concretely:

- `capsule_go_eval` becomes the desk's only JSON envelope; every other desk
  affordance becomes a typed in-cell function (`choir.Message`, `choir.Spawn`,
  `choir.Complete`, `choir.Freeze`, `choir.Verify`, `choir.InspectBundle`) that
  stages an intent for the one reducer.
- The five overlay JSON tool names (`commit_transaction`,
  `inspect_self_development_bundle`, `record_self_development_verification`,
  `record_assignment_result`, `update_coagent`) are deleted rather than
  deprecated, each behind a replay proof.
- The reducer, not a tool, authors the assignment fate; run acceptance stops
  keying on tool names and fails loudly when evidence is missing.
- OpenCode Go and Zen become live providers so the roster can compare models.
- One model-independent prompt and one REPL initialization serve every expected
  roster model with zero output repair.
- The `actuator=tools` branch stays alive but unused (residue R8) until the
  management and research desks also cross.

The mission inherited residue R7 (the tool-retirement debt from mission two's
vocabulary-only rename) and opened R8 deliberately.
**A short glossary, for terms this report uses heavily:** a *tell* is an owner
instruction queued on a document channel for the management desk; a *drainer*
run is the resident agent that consumes queued privileged instructions; a
*sweep* is a periodic reconcile pass that re-drives states matching a
signature; an *arm* is one roster attempt (owner tell through collected
receipt); the *assignment fate* is the reducer-authored lifecycle record of an
assignment (opened → bound → frozen → terminal); a *frozen capsule* is a
workspace snapshot sealed at assignment completion whose remaining cell
execution needs a worker wake; a *constructed CodeRef* is the immutable build
identity a guest computer stays pinned to until a product-path lifecycle
refresh moves it; a *cursor* is a reducer sequence number on the document's
event tape.


## 3. How the mission runs

The Definition encodes a discipline rather than a schedule: every mutation
boundary is frozen first, reviewed by an independent multi-agent panel while
frozen, adjudicated to one outcome, and only then does code move. Four consensus
gates sit at the frozen boundaries (after P0, on the red P3 diffs, on the P4
deletion candidate, on the P5 roster results). Before any of that, the charter
itself survived two panels: a consensus draft panel (13/13 answered) and an
executability review that returned **"Not executable as written; do not
charter"** — the Definition was revised accordingly, the assigned-run
model-selection gap was found during entry reconciliation, and only then was
the mission chartered on its entry gate.

Problem-documentation-first is enforced structurally: every platform defect
found during execution entered the Definition's `problems_discovered` ledger
with evidence, class, and repair state before or alongside any fix. At HEAD
the ledger holds 37 entries: 29 discovered, 5 introduced (four by the
mission's own replay-harness work, one by its roster-harness work), and 3
recorded as repaired — of which only the guest cutover is fully repaired; the
other two say "partially repaired" in their own text, and one was re-opened by
later evidence. Several further discovered entries carry shipped partial
repairs.

## 4. The build: P0 through P4

### P0 — the freeze (2026-09-11, ~18:10–19:40Z)

The first mission commit was code-free, per the Definition's ordering rule. The
freeze artifact (`docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md`)
records four defects with file-and-line evidence:

- **D1 — the eval envelope is not minimal.** `capsule_go_eval` declared
  interchangeable `source`/`code` parameters, an empty `required` list, and a
  silent source-wins fallback; a call with neither parameter executed an empty
  cell.
- **D2 — run acceptance keys on tool names.** The capsule checkpoints were
  built by scanning tool-result events for retired names, so retiring a tool
  silently removed its checkpoint instead of failing the run.
- **D3 — the reducer is a message router, not a settlement authority.** A JSON
  tool (`record_assignment_result`) ran the entire assignment-fate saga —
  digests, slot gates, freeze/revoke ordering, lifecycle CAS — outside the
  reducer.
- **D4 — the verifier slot is unreachable on the assigned path.** Both
  verifier-gated tools required a `co_super_slot` the assigned runtime never
  wrote, and no test pinned either gate.

The same artifact froze the nine-operation mapping table (rows 1–4 are the
capsule file/exec operations deferred to R8 with their in-cell successors
already named; rows 5–9 are this mission's deletions), the prompt/REPL
initialization manifest with its entropy-exclusion list, the falsifiers, the
replay-harness specification, the model-selection mechanism, and the frozen desk
task. The P0-review panel (13 launched across two subruns; 11 completed ok —
one without a final verdict — one provider failure, one timeout) adjudicated
**repair**, and revision 2 of the artifact is the frozen boundary every later
phase cites.

### P1 — providers (2026-09-11, 19:04–19:58Z, parallel-authorized)

The provider contract was frozen before implementation: the model-to-wire-shape
map across OpenAI chat completions, OpenAI Responses and Anthropic Messages;
session identity bound to the durable `RunID` and threaded end-to-end; a
fail-closed rule that refuses to send any request whose identity is empty; a
product User-Agent; credential delivery through the authorized deploy script
only, with a pre-change digest and backup. The wiring commit (`2af02977`, red)
landed the OpenCode Go and Zen adapters with the run-bound identity, and the
deployed live proof exercised one call per wire shape plus the empty-identity
negative. The proof itself surfaced a defect: silent `response.incomplete` and
`response.failed` states on the Responses path, fixed in the same phase
(`856a6014`). The roster's model-selection plumbing
(`model_policy_overlay_id` on `assign_co_super`) landed later the same day
between the P3 and P4 boundaries (`761c4a1e`).

### P2 — the envelope (2026-09-11, 19:58Z)

One commit (`0a80213b`, orange) simplified `capsule_go_eval` to a single
required `source` string with a one-sentence description, and deleted
`CleanGoSource` and all five of its call sites — the markdown-fence stripper
that existed to repair model output. Under the new contract, fenced source and
missing source fail loudly at the schema boundary; nothing is repaired. The
prompt-invariant tests pin that the desk prompt is byte-identical across the
roster models and names the exact in-cell surface with no future tool lie.

### P3 — settlement (2026-09-11, 21:28–21:54Z)

`17c05ac0` (red) made the reducer the sole author of the assignment fate and
added the in-cell freeze/verify/inspect surface: `choir.Complete` stages an
IntentComplete that the reducer settles through the fate saga with the same
identity, digest and CAS semantics the JSON tool used to enforce;
`choir.Freeze`/`choir.Verify` stage their intents; `choir.InspectBundle` reads
synchronously. The acceptance layer was rebuilt to derive the capsule
checkpoints from the durable operation record (bundle digest, operation id,
state, verifier refs) and to add an explicit `failed` checkpoint naming missing
evidence — a completed run without its evidence cannot report an accepted level.
`update_coagent`'s authority checks, typed packet schema and durable update-id
derivations were carried into the staged message path. The P3-review panel (4
selected, 3 ok) adjudicated repair on every successful panelist, and `3ac5cd54`
landed the repairs: acceptance gate, outcome kind, digest binding.

### P4 — replay proofs and deletions (2026-09-11 21:57Z → 2026-09-12 13:31Z)

This was the longest phase, and the review history is the story. The harness
(`93bb82c9`, yellow) implements the frozen spec: versioned fixtures, a
canonicalizer, an effect census, durable receipt storage, and legacy/successor
adapters per operation. The five deletions (`304ae6b7`, red) cut the overlay
JSON tools and made the eval envelope the sole desk tool. Then the consensus
rounds:

- **r1** (11 ok / 2 failed): repair — the tools-fallback prompt still named
  `update_coagent` (`009e3c52`), the inspect host body was orphaned, staged
  intents dropped the actuator mismatch.
- **r2** (11 ok / 1 failed): five repair, four accept — replay row-5/7/9
  receipt mismatches, row-8 `execution_refs`, catalogue digest/schema gaps.
- **r3** (9 ok / 2 failed): repair — and the sharpest finding of the mission's
  self-scrutiny: **the row-5 golden was self-seeded and the recorded build SHA
  was false** (the capture could not have run on the claimed `a907f713` tree).
  The provenance was corrected honestly: the goldens carry `build_sha: 8f987d7f`
  with a qualified restamp note saying exactly which tree the capture ran on
  and why.
- **unstick round** (5 ok; 4 repair-first / 1 recapture-required): rerun
  poisoning, fixed-key identity, an unasserted effect count, corrupt-directory
  pollution.
- **r4** (5 ok; 3 repair / 2 accept) plus a two-panelist spot check that
  returned ACCEPT on the repaired candidate.

The deployed proof (`choir-rlm-replay-deployed-proof-2026-09-12.md`) closed the
phase: on staging at exactly the deployed SHA (`d6b4fab1`, CI
`34695293124`, deployed 13:22:18Z with effects OFF), the Linux-only harness
replayed all five goldens through the real in-cell carrier in 1.64 seconds —
canonical equality on the declared P0 fields, a zero-effect census, a pre-effect
conflict leg on reused identity with changed input, and a fresh new-identity leg
per operation. Two named exclusions carry as residues (row-5 `state`, row-9
`replay`), and fresh-Freeze independence is deferred, not claimed.

## 5. The proving ground: P5 (2026-09-12 → 2026-09-13)

P5 asks the question the whole mission exists for: does the carrier actually
serve a real desk task through a real model, end to end, without output repair?
The frozen task (inspect `GoEvalRequest` field count, write a marker file,
run `go vet`, complete with execution refs) is pinned by digest; the roster CLI
(`roster preflight/start/collect`, plus file verbs for artifacts) gates every
arm on pinned task bytes, the arm's overlay id, the served provider/model, and
— after the review round — the reducer-authoritative assignment fate.

The phase began with construction and immediately found reality:

**The funding and fidelity layer.** The first pilot tell was refused by the
staging snapshot's dirt guard; the routing proof then showed every arm
resolving to the **unfunded base policy** (`deepseek/deepseek-v4-flash`, HTTP
402) because the owner's model choice never reached the assignment. The fix is
fail-closed in both directions: prose-only overlay selection is refused at
assignment open (`c7bbca4d`), and a receipt whose arm was served by anything
other than its overlay fails loudly (`391126cc`). A1's task body was a
paraphrase of the frozen bytes, which produced the strict task-digest gate —
and when the P5-review panel later caught a free-form echo passing as a hash,
`roster collect` was hardened to compare the bound run's served objective
digest against the pinned frozen task digest (`e97bfa9d`, regression-tested).

**The activation layer.** Arms A4 and A5 told the desk nothing happened: no
turn, no run, no inference call. The diagnosis chain is a careful piece of
substrate archaeology: the persistent Super drainer that
consumes owner instructions **completed at 20:27:38Z instead of passivating**,
nothing replaced it, and the activation paths that could restart it are only
the owner's self-development start/retry and boot rewarm (which reactivates
only runs passivated by a process restart). A deterministic rewake caller had
been projected to `state: running` with no execution admission — a phantom
that self-seals because its Active state makes the join a no-op. The owner
decision was requested and the mission parked rather than patched outside its
authority.

**The substrate repair wave (2026-09-13).** With the owner's completion route,
the mission executed a live substrate campaign on the retained computer: the
texture wake strand repaired (`67823ae2`), the capsule-evidence 413 bounded to
the assignment closure instead of the whole object graph (`aac830ce`), the
fresh-mint drainer watchdog added (`a2676517`), the unfunded providers deleted
from the catalogue (`f8db2ed1`, red), the deploy-impact classifier re-pinned to
diff against the deployed staging identity (`9ba782b8`), and then — the step
that unblocked everything — the retained computer's guest cut over to the
repaired build through the product lifecycle path (epoch 913→914, receipt
`01a098f3`, 04:08:22Z), with a later restart to epoch 915. The wake chain
verified live: the guest's first boot sweeps consumed the stranded rewake
**and** both pending roster tells in one committed turn.

**The mechanism proof.** Arm A9 (facts-driven tell, assignment-f67606c6)
delivered what no arm had before: the gateway log at 07:50:20Z shows
`provider=opencode-go model=deepseek-v4.1-flash` serving the capsule's cell —
the first roster arm ever served end-to-end by its own funded overlay rather
than the base policy. The assignment opened, bound, and froze through the
reducer. The frozen-capsule cell-drive strand was then diagnosed and repaired
in the substrate (`caa171b6`: the reducer authors the terminal fate from the
staged pending proposal, wired on restart reconcile and the Super selection
sweep), and the capsule receipt binding was narrowed to authenticity with the
frozen final subject certified once (`7afda8d1`).

**The terminal receipt and its stranding.** Arm A14
(assignment-9ec36ecb, tell `p5-roster-open-arm-20260913-7`) executed all three
task cells through the funded overlay — field count read (5 fields), marker
written and byte-verified, `go vet` run (exit 1, environmental: networkless
capsule, cold module cache) — and staged `choir.Complete` correctly. The
terminal saga progressed `freeze_requested → frozen → revoke_requested`
(cursors 1209–1211) and then **stranded**: no revoked acknowledgement, no
terminal commit, no reports. The worker's own run completed at 13:25:17Z
reporting "Terminal is settled" — a run-state belief that the reducer's fate
did not corroborate. `roster collect` correctly recorded
`needs_human_classify=true`, `pass=null`, and quarantined the receipt:
mechanism proven, terminal disposition not reducer-authoritative.
**The P5-review panel** (13 launched; 11 completed) adjudicated
PASS-WITH-FINDINGS on the mechanism, with two strict FAILs holding only the
full four-id gate. The dissent went further than the tally: the codex
panelist held the served task bytes wrong and the terminal evidence
inconsistent; the sol panelist flagged stale roster evidence and missing
prompt-digest evidence. The majority confirmed the strand by reducer
inspection and returned findings: **F1** the served objective was 751 bytes
against the frozen 1051 (repaired in-sweep with the digest gate), **F2** the
tally is one-of-expected-ids (the sibling supervision channels are dead or
degraded), and **F3** the terminal-saga strand itself. Two owner-facing facts
belong beside the receipt: it was collected under a degraded gateway health
report (dolt/ollama/runtime probes unhealthy in the guest), and the bound run
consumed roughly 697k input tokens for a four-step task.

With that, the mission escalated rather than patched:
`docs/memo-activation-wake-authority-substrate-2026-09-13.md` records the
structural assessment — four strands in one family inside 24 hours, all
sharing one cause: *a lifecycle transition happens but no durable wake or
retry authority is minted for the continuation.* The owner decision requested
names three consequences, of which the owner must pick one: authorize the
wake-authority substrate repair (transition-minted recovery occurrences, one
consumer, run-terminal ≠ fate-terminal), repair the sibling channels, and
rerun all four expected models; or owner-amend the roster scope and evidence
floor; or settle the Definition `blocked_incomplete`.

The mission then shipped the first wave of that design inside its own
authority: the fate-transition watchdog with a minute-scale window and
`revoke_requested` resume (`6a69878a`), coverage for revoked-capsule strands in
the resume paths (`6a869d7f`), roster collect gated on the
reducer-authoritative fate (`4bbb9266`), and exact strand-state logging on
failed terminal reports (`def29358`). Two further arm cycles (assignments
`121ae9fe`, `e8592727`) then stranded inside the terminal saga's revocation
step itself — non-deterministically, with the guest infrastructure churning
underneath — making five and six in the family. One caveat belongs here: the
arm live-status receipt committed at `3680c125` predates these two strands and
reads more optimistic than the ledger ("the strand recovered each cycle so
far"); the Definition's `problems_discovered` entries are the authoritative
record, and this report follows them.

## 6. Where it stands at HEAD

- **Implemented and locally audited:** P0–P4. The completed-phase audit ran at
  `main@5f34baf1` (not HEAD) and re-verified 13 deliverable claims from code
  and test output; every claim holds, with the desk-scoped `update_coagent`
  exception resolved in P3-parity's favor and the Linux-only replay proof
  held by artifact. Outstanding from the Definition's evidence floor: the
  deployed acceptance-rebuild negative, the live registry observation, and
  the deployed engineering-assignment acceptance with accepted ids — all P6
  obligations.
- **Proven, quarantined:** the P5 mechanism. The arm chain
  tell → desk compliance → structured overlay argument → assignment
  open/bind → funded-model cells → staged Complete is landed, observable and
  reproducible; the terminal disposition is not reducer-authoritative, the
  receipt stays quarantined, and the P5 gate stays open.
- **Not started:** P6 landing (intermediate deploy, deployed replay/roster
  proofs, atomic registry move, R7 closure, Definition settlement).
- **The live ask:** the owner decision on the wake-authority substrate repair,
  plus the fate of the sibling supervision channels (one dead at scale from
  unbounded caller memory, one pre-genesis) that gate the full four-id roster
  tally. Note for the cross-checking reader: the Definition's `now` card
  (slice/question/next_action) still carries its 2026-09-12 three-layer
  blocked text and lags the 2026-09-13 problems ledger that this report
  narrates.
- **Deployment/CI at report time (read live, not yet a durable receipt):**
  staging `https://choir.news` serves `def29358` (deployed
  2026-09-13T17:39:04Z, effects OFF; the guest runs the substrate-repair build
  9ba782b8 through its constructed-computer identity); CI is green on HEAD
  `3680c125` (run `34775505983`).

## 7. The defect ledger, in families

The 37 `problems_discovered` entries cluster into families worth naming,
because the clustering is itself the mission's argument:

1. **Activation-transitions-without-retry-authority (six strands).** Fresh-mint
   dispatch, texture owner-instruction wake, frozen-capsule cell drive,
   terminal-saga revocation (×3). One cause, escalating treatment: two local
   sweeps repaired in source (texture wake, frozen-capsule fate resume), one
   worked around through the designed owner path (run cancel + restart,
   epoch 915), and the structural memo escalated the class to the substrate
   design.
2. **Replay-harness vacuity (introduced by the mission's own harness).** Four
   entries — self-seeded goldens, false build provenance, rerun poisoning,
   vacuous corrupt-legs and unasserted effect counts — all caught by the
   mission's own review rounds and repaired or honestly narrowed with recorded
   residues; the fifth introduced entry is the roster-harness work-item mint
   (family 3).
3. **Roster fidelity gates (the harness learning to fail loudly).** Task-body
   paraphrase, prose-only overlay selection, base-policy service, free-form
   task-hash echo, orphaned work items minted per tell — each became a hard
   gate in `roster collect`.
4. **Evidence-surface bounds at scale.** The capsule-evidence route 413'd on a
   whole-computer object bound; the sibling texture caller's unbounded run
   memory grew until chatgpt rejected it. Same class, two surfaces; one
   repaired (bounded to the assignment closure), one recorded.
5. **Infrastructure truth-tellers.** The pre-genesis admission refusal loop
   (one computer, ~11k refusals/day, attributed), the deploy-impact
   classifier diffing the wrong base, guest URL mobility during deploys,
   computer-owned startup config naming a deleted provider, the unbounded
   Definition front-matter truncation (the working Definition's own card was
   silently clipped for ~40 commits and nothing in the repo detects the
   class), the assigned-fate join's cancellation race, and the
   freeze-classifier's always-reject default — recorded so acceptance probes
   resolve the route at use time.

## 8. Independent review history

| Panel | Date | Composition | Verdict |
|---|---|---|---|
| Charter draft (mission 3) | 09-11 | 13 agents, 13 ok | produced the Definition draft |
| Executability review | 09-11 | 13 selected, 12 ok | **not executable as written** → revised, then chartered |
| P0-review (two subruns) | 09-11 | 13 launched; 11 ok (1 no verdict), 1 failed, 1 timeout | repair → freeze revision 2 |
| P3-review | 09-11 | 4 selected, 3 ok | repair (all successful panelists) |
| P4-review r1 | 09-11 | 13 launched; 11 ok, 2 failed | repair |
| P4-review r2 | 09-12 | 12 launched; 11 ok, 1 failed | repair (5 of 12 findings) |
| P4-review r3 | 09-12 | 11 launched; 9 ok, 2 failed | repair (golden provenance corrected) |
| P4-review r4 + spot | 09-12 | 5 ok + 2 ok | repair, then ACCEPT |
| Model-policy design | 09-12 | 13 lenses, 10 ok | divergent; runtime capability-alias direction recorded |
| P5-review | 09-13 | 13 launched; 11 ok, 2 failed | PASS-WITH-FINDINGS; receipt quarantined |
| P5 roster-CLI design | 09-13 | 13 launched; 11 ok, 2 failed | thin CLI through owner management inputs |

Panel-health metadata is recorded with each run: the 13-agent default panel
reliably yields about ten usable opinions, with the same two provider-quota
failures recurring — a noted future boundary improvement, not evidence.

## 9. Evidence inventory

| Artifact | Path |
|---|---|
| P0 freeze artifact (rev 2, frozen boundary) | `docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md` |
| P1 provider freeze | `docs/evidence/choir-rlm-engineering-carrier-p1-provider-freeze-2026-09-11.md` |
| Completed-phase audit | `docs/evidence/choir-rlm-engineering-carrier-completed-phase-audit-2026-09-12.md` |
| Replay deployed proof | `docs/evidence/choir-rlm-replay-deployed-proof-2026-09-12.md` |
| Replay goldens + manifest | `docs/evidence/rlm-replay/goldens/` |
| Roster arm evidence (historical — ends at arm A5) | `docs/evidence/rlm-roster/p5-roster-evidence.md` |
| Frozen desk task + overlays | `docs/evidence/rlm-roster/p5-frozen-desk-task.txt`, `p5-*.overlay.toml` |
| Terminal roster receipt (quarantined) | `docs/evidence/choir-rlm-engineering-carrier-roster-receipt-2026-09-13.json` |
| Arm live-status receipt (predates strands 5-6; ledger is authoritative) | `docs/evidence/choir-rlm-engineering-carrier-arm-status-2026-09-13.md` |
| Wake-authority escalation memo | `docs/memo-activation-wake-authority-substrate-2026-09-13.md` |
| Model-policy consensus synthesis | `docs/reports/choir-model-policy-consensus-2026-09-12.md` |
| Charter consensus + review records | `docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md`, `...-review-2026-09-11.md` |

## 10. Residues, rollback, heresy delta

**Residues.** R7 (tool/operation retirement ownership) is this mission's own
charter and closes at P6 landing. R8 (`actuator=tools` deletion) stays open
until the management and research desks cross. New owner-ratified residues
R9/R10 record the provider and Super heresies from the model-policy work.
Replay residues are named, narrow, and enforced by the driver guard: row-5
`state`, row-9 `replay`, fresh-Freeze independence deferred.

**Rollback.** Git revert of the mission's commits remains the repo rollback
path. The retained computer's guest identity moves only through the product
lifecycle path (lifecycle refresh receipts), never platform-side surgery —
product restore is a separate forward transaction, and the guest stays pinned
to its constructed CodeRef by design.

**Heresy delta (as recorded in the ledger).** Discovered: 29. Introduced: 5
(four replay-harness defects and one roster-harness defect, caught and
repaired or narrowed in-mission). Repaired: 3, with only the guest cutover
fully repaired and several further discovered entries carrying shipped
partial repairs (texture wake strand, capsule-evidence bound, frozen-capsule
fate resume, receipt binding, task-digest gate, fate watchdog).

## 11. Commit inventory

107 commits from charter `47f85841` to HEAD `3680c125`:

| Class | Count | Purpose |
|---|---|---|
| `green` | 64 | Definition reconciliation, evidence receipts, problem documentation, panel records |
| `orange` | 33 | Providers, envelope, settlement surface, replay harness and repairs, roster CLI, substrate repairs |
| `red` | 6 | Protected-surface moves: provider wiring (`2af02977`), five-tool deletion (`304ae6b7`), fallback prompt repair (`009e3c52`), settlement (`17c05ac0`), P3 repairs (`3ac5cd54`), unfunded-provider deletion (`f8db2ed1`) |
| `yellow` | 2 | Replay harness substrate, shared-prompt revision (optimization pressure) |
| `fix(ci)` | 1 | Race-shard contract |
| `revert` | 1 | Undo of a pilot tree regression |

The terminal commit is a docs receipt, not a landing commit: the mission is
in flight at its P5 gate, and this report records it exactly there.

---

*Method note: this report was drafted from the repo's durable record
(Definition, evidence artifacts, commit history, panel outputs) at HEAD
`3680c125`, then reviewed by the author and an independent agentic-consensus
panel (13 launched: 7 ACCEPT or ACCEPT-WITH-FINDINGS, 2 REWRITE, 2 provider
failures, 1 timeout, 1 no verdict; prompt and outputs under
`.agentic-consensus/report-review/`, run at draft digest
`dd230534bab872df`). The convergent findings — clock-label errors, the
arm-status receipt's optimism, the stale `now` card, the fresh-mint
workaround wording, dissent sharpening, cost/health disclosure, and the
evidence-floor gap list — are folded into this revision.*
