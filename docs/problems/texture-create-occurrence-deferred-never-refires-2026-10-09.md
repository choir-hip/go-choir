# A new document's first Texture turn waits for the next event (2026-10-09)

Found by the Texture acceptance suite (Gate 1 exit item; receipt `evidence/texture-acceptance-2026-10-09T18-23-11-189Z.json`), second run on
staging build e3f2d560, disposable probe account `4b579d04…`
(`computer-fd097d5d…`, VM `vm-47daf854…`). Mutation class of the eventual
fix: red (actor delivery). This doc comes first; no fix is in it.

## Evidence (trace, not inference)

Guest console log and the Caddy access log, all times 2026-10-09 UTC:

| Time | Event |
|---|---|
| 18:23:14 | Suite creates document `e053b60e…` (`POST /api/texture/lifecycle-documents`, 201). |
| 18:23:20, 18:23:24 | Texture activation: tool loop iterations 1 and 2. |
| 18:23:28 | `dispatcher: deferred <owner>/texture:e053b60e…/07ebb376… discards 5 emitted update(s)`, then `deferrals=1 cause=actor: defer unprocessed occurrence: actorruntime: Texture activation returned without disposing exact trigger`. |
| 18:23:28 – 18:28:20 | Nothing for this actor: no further deferral lines (`deferrals=2` would appear on any re-fire that deferred again), no activation, no tool loop. |
| 18:28:20 | Suite's owner revise arrives (`POST …/revise`, 202, Caddy log). |
| 18:28:30 | `texture cell authored turn (run 471cd649…)`: base `5409efbf…` → revision `d8444a16…` (the first appagent revision). |

After 18:28:30 the document is idle (no `agent_revision_pending`), with
one appagent revision. `GET /api/runtime/obligations` on the guest at
18:34:49 reports no wakes owed, 4 passivated runs and 3 open work items,
all from 2026-09-23.

## What is established

1. The create-time occurrence deferred at 18:23:28 and was not handled
   again until the owner's revise, 4 min 52 s later. The first draft of a
   new document therefore waits for the owner's next action.
2. The deferral discarded 5 emitted updates. The dispatcher's comment says
   the event re-fires and the handler re-emits them; here it did not
   re-fire for ~5 minutes.
3. The dispatcher arms a due-timer from `NextDue` and the first deferral's
   backoff is 500 ms (`internal/actor/dispatcher.go` `deferralBackoff`),
   so a re-fire was expected around 18:23:29.
4. The SL "what is owed" surface does not see this: deferred actor-log
   events are not in it, so it answered "nothing owed" while a Texture
   occurrence was stuck. That is an SL gap (O1: no obligation without a
   visible driver).

## Hypotheses (not yet confirmed by trace)

- **H1, re-fire suppressed.** The dispatcher did re-fire after 500 ms,
  but the handler returned without deferring again or logging (for
  example, the parked run was still bound and the reconcile path took a
  silent no-op), so the event stayed unprocessed with no new not_before.
- **H2, not_before not re-armed.** `DeferUpdate` or `NextDue` did not
  yield a due time for this event, so only a new event for the actor
  (the revise) made it pending again.
- **H3, awaited wake discarded.** The activation parked to wait for a
  wake that one of its 5 discarded emissions would have produced, so the
  awaited wake could never come; only an unrelated owner event moved it.

## Next observation

Read the actor log row for update `07ebb376…` on the guest (defer_count,
not_before, processed) and the 5 discarded update kinds. That needs a
product read path, not SSH into the guest: extend the obligations surface
with deferred and unprocessed actor-log events (count, oldest, defer
count, next due). That extension is also the SL fix for gap 4. Then
reproduce with a focused actor-dispatcher test.

## Harness note (separate, not product)

The suite's T2 failed on this run for a harness reason too: it waited for
`agent_revision_pending === false`, but the field is `omitempty`
(`internal/textureowner/texture.go:163`), so "not pending" is an absent
field and the check never passes. The first run failed on 5-minute access
tokens with no renewal. Both are fixed in the spec; neither changes the
product finding above.

## Gateway trace (added 18:55Z)

Gateway journal on Node B for this VM, 18:23–18:29:

- 18:23:15, 18:23:20, 18:23:24: three model calls for the create
  activation. The first two stop on `tool_use`; the third, at 18:23:27,
  stops on `end_turn` with 447 characters of text and no tool call. The
  Texture desk ended its turn by talking instead of applying, so the
  trigger stayed undisposed and the occurrence deferred.
- **No model call from 18:23:27 to 18:28:20.** The deferred occurrence was
  never executed again; a re-fire that reached execution would call the
  model.
- 18:28:20: a fresh call (3 messages) the moment the owner's revise
  arrives; it applies at 18:28:30.

This refutes a fourth hypothesis I considered (a re-fired activation
blocked for 5 minutes on a model call: the two 5-minute clocks, the
suite's wait and the deferral, started 15 s apart). It adds:

- **H5, silent consumption.** A re-fire around 18:23:29 found the
  occurrence's fate terminal (or a "durable foreign" envelope) and
  returned `nil, nil`, which incorporates it without a log line. Then the
  create's first draft is not late but lost, and the owner's revise
  produced the only appagent revision.

H2 (not re-armed) and H5 both match the silence; H1 (re-fire deferred
again) does not, since every deferral logs. The observer upgrade
(690ef1ee, actor section of the obligations surface) separates them on
the next reproduction: H2 shows the update still deferred or due; H5
shows nothing unprocessed while the document has no first draft.

Two product questions follow, whatever the mechanism: should a Texture
turn that ends without applying on a create wake be retried by the desk,
or reported to the owner as "Texture replied without revising"? And the
silent `nil, nil` returns in the Texture occurrence handler consume
obligations with no visible fate (O1).
