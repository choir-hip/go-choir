# Cut 1 breakage log

One log per cut window (docs/re-engineering-decisions-2026-10-12.md §12.4):
problems found and fixed in the same commit during Cut 1 of
`docs/archive/definitions/choir-reeng-cut1-authority-2026-10-12.md`. Problems that
are not fixed in the commit that finds them get their own problem doc.

## B1. A crash boot starts a new research run that takes a pre-boot Cast

Found 2026-10-13 by the Cut 0 tracer, run `c0-fast-1` (build 287734c5,
bundle sha256 2306081c…, trajectory 2b5b9631). Invariant I3 failed with ten
post-boot dispatches; the first two are the defect.

Trace, from the bundle's lifecycle events and runs:

1. 05:11:27 the owner restart route restarts the computer while research run
   3c3ea752 is in a cell; boot at 05:11:34 has no planned-restart marker.
2. Boot passivation (`passivateInterruptedActivations`) parks 3c3ea752 and,
   for it, opens a spawned work item (lifecycle event seq 10,
   `lifecycle-open-spawned-work:3c3ea752…`).
3. That work item's wake is minted after the boot, so the actor handler's
   crash-boot refusal (`preBootWorkOccurrence`, which only refuses
   occurrences recorded before the boot) lets it through.
4. A new research run 2127efc8 starts at 05:11:35 and takes Texture's second
   pre-boot Cast, control packet 93b04b27 (seq 11,
   `bind-control-delivery:2127efc8…:93b04b27…`). It runs until 05:19:42 with no
   owner input.

So a crash restart resumed work, against the owner rule (CLAUDE.md,
"Restarts End Work"). The accept-1 bundle shows the same first link (a
spawned work item opened one second after boot for a pre-boot Texture run).

Fix (landing L1, same commit): a markerless boot no longer passivates. It
cancels every pre-boot pending, running, parked or blocked run with fate
`interrupted_by_restart`, opens no work, and wakes no one
(`closePreBootRunsAfterCrash`; Engineering cancel wakes and the boot
Engineering recast are suppressed on a crash boot). Planned boots keep the
park-and-resume path. Pre-boot pending control packets stay pending until
the owner's next act; Cut 1 landing L3 deletes packets.
Test: `TestCrashBootClosesPreBootRunsWithFate` (fails on the old boot:
runs passivated, parked and blocked runs left open).

## B2. A crash boot misses an open run behind 1,024 newer runs

Found 2026-10-13 by the convergent plan review (codex), confirmed by test.
Boot lists the owner's runs with `ListRunsByOwnerStates(..., 1024)`, and
that list applies its limit to run headers before it filters states
(internal/store/graph_store.go:1039, :1057). An old pending, running,
parked or blocked run behind more than 1,024 newer finished runs is never
seen, so L1's claim "every pre-boot run is closed" fails on any computer
with a long history (the owner's computer takes every cut).

Fix: boot's owner-scoped list reads every run header with no LIMIT
(`store.ListAllRunHeaders`, through `ListAllObjectRefsByKindOwner`;
headers carry no bodies and only matching runs are loaded). The first fix
(877689e1) passed a limit of 1<<30 instead; CI's race shards then died
"runtime: out of memory" in the boot tests, because the engine sizes a
LIMIT's top-N buffer up front. That build never deployed (CI run
38352848069 failed); a huge LIMIT would have crashed every computer at
boot. Test: `TestCrashBootClosesOpenRunBehindManyFinishedRuns`
(one old running run behind 1,030 finished runs; failed with the run
still running before the fix).

## B3. A cell that starts with a type, var or const declaration fails

Found 2026-10-13 by the desk cell-error analysis of the Cut 0 bundles
(docs/archive/reports/reeng-desk-cell-errors-2026-10-13.md, mechanism M1): 22 of
67 failed cells, the largest class, and it repeats (the model's next cell
often has the same shape). yaegi picks file or statement parsing from a
cell's first token, so `type T ...` followed by statements fails
"expected declaration"; the normalizer guarded only a leading `func`.

Fix (same commit): `guardLeadingFuncStatement` also prefixes `;` for a
leading `type`, `var` or `const` (only cells that do not parse as a file
reach it). Test: `TestCellStartingWithDeclarationRunsAndPersists` (failed
"expected declaration, found ps" before the fix). Open from the same
report: M3 (`degradeBoundDefines` turns a retyping `x :=` into a failing
assignment), M4 (record fields the prompts never teach), M7 and M2a
(yaegi compile defects, two of which kill the worker), and the prompts'
false claim that a failed cell poisons the session.

## B4. A records wake reactivates a parked Texture run and nobody runs it

Found 2026-10-13 by the management scenario c1-l3b-mgmt-1 on 49092e2f
(bundle ~/.config/choir-qa/traces/c1-l3b-mgmt-1/bundle-a1.json, sha256
ab0561ed…). Timeline from the trace: Texture cast to management at
07:45:52; management's final Report at 07:48:36 started Texture run
258b508d, which read it and committed a decide turn with a Cast to
research at 07:49:20 (no new revision), then parked
(`passivated_reason durable_transition_success`, mutation sleeping).
Research's final Report reached `texture:<doc>` at 07:56:22. At
07:56:22.9 run 258b508d went to `pending` with
`actor_reactivated_from_passivated` and `request_source update_coagent`.
No loop event followed in the four minutes before the tracer stopped the
computer; Texture's cursor stayed at 1 under head 2.

Cause, from code: the records wake (`handleInboxRecord`,
internal/actorruntime/handler.go) calls `ReconcileAgentWake` and discards
its result. When the document head still equals the parked run's
revision (the turn was a decide, so the head did not move),
`reactivatePassivatedTextureRun` selects that run, sets it pending and
returns it, expecting the calling occurrence to execute it ("the current
coagent_result occurrence is the execution authority"). The coagent path
does so; the records path does not. A fresh run (head moved) is
dispatched by `submitTextureAgentRevisionRun`, which is why c1-l3a-story-1
(every turn an apply) never hit this. Any records wake after a turn that
left the head unchanged stalls Texture, and the pending run then
suppresses every later wake as the active run.

Fix: the inbox_record occurrence executes a run that this wake
reactivated, as the coagent path does. Mutation class orange (Texture
activation dispatch; no canonical write path changes).

## B5-B8. Liveness gaps on the records routes (C1-core close review)

Found 2026-10-13 by the C1-core close panel (claude opus,
gemini-3.8-flash, deepseek-v4.1-flash; devin was refused a tool call;
`.agentic-consensus/c1-core-close-2026-10-13/`, prompt
`.agentic-consensus/c1-core-close-prompt-2026-10-13.md`). Two of three
answers said "holds with follow-ups", one said "reopen". Each staging
receipt ran while the owner's work item was still open and while every
research run ended with a final Report, so no receipt crossed these
boundaries. All four block a long-lived story, the shape Autopaper
needs.

**B5. Once its work item closes, a Texture is no longer woken by
records.** (deepseek, confirmed in code and by a throwaway probe.)
`ValidateActivationAuthority` (internal/textureowner/texture_controller.go:392-400)
requires the run's `lifecycle_work_item_id` to name an open work item. A
records-started run on a Texture whose owner work item has completed
carries no work item (aef01cef permits the submit). Its initial dispatch
fails "validate Texture activation work: record not found", and the run
stays pending. Second shape: `agentMutationComputerID`
(agentcore/runtime_persistence.go:170, textureowner/coagent_injection.go:20)
scopes a mutation to the computer only when the run has a work item. A
workless records run's parked mutation is invisible to the controller's
computer-scoped reads, so a records wake mints a fresh run instead of
reactivating the parked one. A parked run whose work item has closed fails
the same authority check. On the records route, work is the records and
not a work item; this authority check predates that.

**B6. A round stays open when its recipient's run ends without a final
Report.** (claude, gemini.) Only the recipient's final Report or the
crash boot (`closeOpenRoundsAfterCrash`) closes a round. If a research
run fails, is cancelled, or ends without a final Report, nothing is
addressed to Texture and nothing wakes it. `recheckRecordsInbox`
re-wakes only the run's own desk, and only for late records. The round
stays open until the next crash restart.

**B7. Passivation's records re-check is dead code.**
(gemini.) `passivateIdleToolLoopRun` (runtime.go:3670) calls
`recheckRecordsInbox`, which returns at once for any non-terminal run,
and a parked run is not terminal. A record that reaches a Texture while
its run is active finds the desk busy and wakes nobody (D4). When that
run parks, the re-check that should start the next turn never runs, so
the record waits for an unrelated wake.

**B8. The B4 fix can strand a reactivated run on retry.** (claude,
gemini.) If execution after reactivation defers, the occurrence is
retried, and `reconcileAgentWakeLocked` finds the pending run with valid
authority and returns nil. `handleInboxRecord` then acknowledges, and
the run stays pending with no executor: B4 again, through a narrower
window.

Fix plan (one landing, then a scenario that crosses the boundaries: an
owner request whose work item completes, then a second record to the
same Texture, and a research run that fails without a final Report):
- B5: the records route is the work authority. A Texture run started or
  reactivated for unread records is marked `records_route` and, when its
  work item is closed or absent, carries none; activation authority
  accepts a records-route run without a work item. Mutations of
  lifecycle runs are scoped to their computer whether or not they carry a
  work item.
- B6: when a records-desk run ends (failed or cancelled, not a crash)
  while it is the recipient of an open round, the runtime appends a
  system closure Report, final with reason `run_ended:<state>`, which
  wakes the caster (unlike the crash closure, which wakes nobody).
- B7: the re-check treats a parked run like a completed turn.
- B8: an inbox_record occurrence that finds a pending reactivated
  Texture run with nobody executing it runs it.

## B9. A records-route Texture cannot write a revision through ApplyTexture

Found 2026-10-13 by the research scenario c2-search-1 on 6e9c469e (B5's
deployed check). The B5 fix worked: when research's final Report reached
a Texture whose work item had closed, a fresh records-route run started
(records_route true, no work item). Every revision it staged then failed
"create Texture revision: lifecycle work identity missing"
(internal/textureowner/tools_texture.go:386-389, the ApplyTexture
materializer), and after about eight attempts the run parked
(idle_actor_passivate) with Texture's cursor at 0. A decide turn would
have committed; an apply could not.

Disposition: the ApplyTexture materializer is deleted by Cut 2 item (iv),
Texture's Write verb, which settles work only through the turn runtime
(open work item or none, B5). Its test covers Done, then Write with Done,
on a records-route run whose work is already completed. No separate fix
on the ApplyTexture path; the research scenario reruns on the build that
carries Write.
