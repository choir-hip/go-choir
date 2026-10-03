# S0m finding: "desk authors no act" is a contract split + mask, not agency

**Date observed**: 2026-10-03
**Computer**: `computer-03335285269bdba4f94377e56879f9e6` (owner guest,
`candidate-fleet-e15cb89f25d963c220319b7b`)
**Deployed**: autoputer `4dbb4a5a` (post stranded-bound-rebind repair)
**Mutation class**: green (observation/boundary documentation only)

## Symptom

The prompt-bar submit mints a live trajectory and the texture desk cell
**dispatches and runs** — `running_runs > 0`, a `texture_turn_committed`
lifecycle event commits — but the turn completes **with no authoring
act**: no `choir.Ask`, no `choir.Resolve`, no `choir.Note`, no
`choir.Cast`. The trajectory's `updates` stays empty; nothing binds a
research carrier; nothing resolves an ask.

Observed on trajectories `d3061b0b` (stranded-bound probe) and
`17b4883f`/`cdf50edc` (canary): desk runs, commits a turn, authors
nothing.

## What is repaired (this is NOT a delivery defect)

The full dispatch + delivery substrate is now landed and verified:

- **Boot-time initial_dispatch** — `ae47c8a4` actor-wake outbox `ogKindRun`
  case: a pending minted run owes its agent a durable activation wake.
- **Post-boot desk-mint starvation** — `ebfd2e98` retry on deadline-exceeded.
- **Restart-recast supersede** — `39e924aa` durable-invalid supersede
  incorporates.
- **Stranded-bound rebind** — `c31bf43a` (this deploy): pure unbind clears
  `DeliveredAt` so a freed packet re-enters the pending scan; union
  liveness oracle (`getRunForComputer`) stops freeing live passivated
  carriers; tri-state claim fate settles completed carriers;
  `replay.Completed` suppresses discharged-turn re-mints.

Result: desks that used to stay `pending` forever now execute. The wedge
moved from *delivery* to *agency*.

## Panel verdict (divergent, 2026-10-03 — claude / muse-spark / gemini38 /
## devin + direct code verification)

The "desk runs but authors no act" reading was **wrong in mechanism**. Four
independent framings converge:

### 1. `consumeIdleTextureTrigger` is a masking false-label
`tools_desk.go:563-625` writes reason `"desk cell completed with no authoring
act"` whenever the cell staged zero `IntentTextureApply`. It does **not**
inspect `IntentAsk`/`Note`/`Resolve`. A cell that staged only an `Ask` reads
identically. The string cannot distinguish "model authored nothing" from
"model authored a non-ApplyTexture act". Worse (gemini38 framing 3): the
consume fires after **any** `desk_go_eval` with `Error==""` and no
`IntentTextureApply` — an exploratory first cell (read doc, prep) commits
`no_worker_needed` and ends the run before a second cell can stage the act.
The trigger-disposal fix (#5) short-circuits the multi-cell notebook the
prompt requires.

### 2. The demanded act was prompt-forbidden (contract split)
The texture overlay (`run_system.yaml`, `revision_policy.yaml:59`) routes ALL
research through `ApplyTexture{controls:[{open_researcher:true, packet:...}]}`
— "Research follow-ups ride the SAME control mechanism that opened the desk"
and "Direct choir.Message/choir.Note to a research desk is rejected by the
lifecycle queue." `choir.Ask` is permitted only as a **follow-up to an
already-bound research desk** (`texture.yaml:76-77`, `run_system.yaml:19`).
The stranded-bound probe demanded a bare `choir.Ask("research", …)` first —
the desk's prompt contract forbids exactly that.

### 3. The demanded act was also store-rejected (addressing precondition)
`choir.Ask("research",…)` → `directiveTargetAgentID` (`rlm_reduce.go:1688`)
resolves `research`→`research:<docID>` → `GetAgentByScope` → `ErrNotFound`
because `research:<docID>` is only minted by an `ApplyTexture` controls
`open_researcher` turn. Even a perfectly-agenced model calling Ask-first gets
a commit-time reject. Addressing requires a prior bound desk.

### 4. Ontological split (gemini38 framing 4 — the real question)
Texture was never cut over to record-native peer messaging. S0m migrated the
delivery plumbing under `management_controller`, but Texture's contract stays
document-carrier controls. A bare texture `choir.Ask` is arguably an
architectural heresy — Texture is the document owner, not a tape peer like
Management.

## Corrected boundary

**Not** "model agency" and **not** a 4th delivery defect. The rebind re-proof
cannot be driven by a bare texture `choir.Ask`. Options (a doctrine choice,
not a patch):
- **(i)** Drive the rebind leg through a record-native peer caller — a
  Management-desk `Ask`→carrier→kill→rebind — decoupling rebind verification
  (substrate, already unit-tested) from texture authoring. Cheapest, keeps
  texture out of the loop.
- **(ii)** Re-proof through texture's *designed* path:
  `ApplyTexture{controls:open_researcher}` → bound research → `Ask`
  follow-up → Reply → Resolve. Exercises the full record-native loop but is
  the heavy path.
- **(iii)** Accept texture-Ask as out of S0m scope; verify rebind via a
  deterministic in-store harness (inject a control, terminalize its carrier,
  assert rebind).

The `consumeIdleTextureTrigger` mask is a real latent defect regardless of
which proof path is chosen — it manufactures "no act" for non-ApplyTexture
authored cells and can pre-empt a multi-cell turn. That is a separate
substrate item (red, Texture canonical writes), worth its own problem doc if
pursued.

## Boundary for S0m

Blocks the finish-acceptance chain — but the correct re-proof instrument is a
doctrine choice (rebind via Management `Ask`, via texture's designed
`open_researcher` controls path, or via a deterministic in-store harness),
NOT a bare texture `choir.Ask`, which is both prompt-forbidden and
store-rejected. All delivery invariants are green; `consumeIdleTextureTrigger`
is a confirmed latent mask worth its own substrate fix (red, Texture
canonical writes).
