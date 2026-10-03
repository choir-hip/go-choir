# S0m Record-Native Messaging — Station Checkpoint

Written October second, 2026. This letter covers the S0m station of the
supervised self-development metamission — the work to cut desk-to-desk
messaging from raw channel envelopes to typed commitment records — and the
proof work that ran on staging this afternoon and evening.

## The short version

S0m's delivery layer is landed and mostly proven. The day's real work was
hunting down two delivery defects that made desk messages silently
unreliable, and one acceptance gap — the "mechanical resolve" — that turned
out to be a desk that simply never took the final step, not missing
machinery.

## What the station was doing

The five desks used to talk through channel envelopes — raw messages that
could arrive, get dropped, or deliver twice. S0m replaced that with typed
commitment records: a precommit stakes a question, a report carries the
answer, a resolve closes the loop, a disagreement preserves a dissenting
verdict, and directives carry operational acts. Every desk act becomes a
ledger event that can be scored. Delivery became a derived packet instead of
a parallel envelope, so there is exactly one path and no dual delivery.

## Two real defects, both fixed

The first was a dead letter. Channel mail addressed to a texture desk got
written durably and then silently dropped — it looked sent, and it never
arrived. The fix rejects that mail at reduce time, so a refused send fails
loudly instead of vanishing.

The second was subtler and worse. A packet bound to a run that finished
before consuming it became invisible. The pending list skips bound packets,
so the stranded packet could never wake its desk — and the only repair ran
inside the desk's own activation, which the invisible packet could never
trigger. Bound meant invisible meant never repaired: a deadlock. On one
staging trajectory this had already happened ten separate times — ten
questions asked, ten answers that could never arrive. The fix moves the
release to the moment a run terminalizes: a dying carrier now frees its
unconsumed packets and pokes the desk to pick them up. Past the retry cap
the packet fails as exhausted, which is a scored failure, not silence.

## What is still open

The acceptance wants an ask to resolve mechanically when texture consumes
the report. That is not implemented. Consuming the report marks it
incorporated but never mints the resolve. I initially thought this needed
new scoring machinery. On a closer read the resolve verb already exists —
texture can close its own ask — it is simply that the desk never called it.
That is a model-agency gap, the same shape as the escalate probes: the path
works, the model didn't take the step. Whether the acceptance truly means
auto-derive-on-consume, or the desk explicitly resolving, is the design
question I put to the consensus panel.

The stranded fix also only stops new wedges. The ten already-stranded
packets predate it; their carriers are gone, so they will stay wedged unless
a backfill sweep is added. That is one of the questions out for consensus.

## Deployment reality

The stranded fix reached staging slowly because of how the deploy pipeline
gates. Documentation-only pushes both skip the deploy and cancel whatever
code build is in flight — so a doc commit pushed mid-deploy killed it twice.
The fix only landed once a push carrying a real code path ran clean. The
deployed proof — driving a fresh ask, killing its carrier mid-delivery, and
watching the desk recover — is staged in a probe script and runs once the
new build confirms on staging.

## The one thing to remember

The failure mode to carry forward is the deadlock shape: a bound packet
that can never wake the thing that would unbind it. When a repair lives only
inside the path it is meant to rescue, it is unreachable exactly when it is
needed. Repairs that depend on the broken thing recovering itself do not
work.

## Names and receipts

Goal file: docs/definitions/choir-appdev-s0m-record-native-messaging-2026-10-01.md.
Metamission: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md.
Texture-mail reject: commit b18f3baf. Stranded-bound fix: commit c9180cd3,
regression test TestLifecycleRunTerminalizeReleasesStrandedControlAndRewakesDesk,
assertion repair e824826a. Problem docs:
docs/problems/s0m-channel-mail-texture-dead-letter-2026-10-02.md and
docs/problems/s0m-stranded-bound-control-deadlock-2026-10-02.md.
Deployed proof probe: scripts/s0m_stranded_bound_probe.mjs.
Trajectory under test: 0bdcbf61-9af0-5b49-b80f-f698e8df7fd5.

## Addendum — the mint blocker, repaired (October third)

A fourth defect surfaced on the stranded-bound acceptance probe and is now
repaired: the desk cell that should mint on a fresh `Ask` never came into
existence, because its durable activation commit starved. After boot, the
persisted run drains roughly two thousand re-armed deadline rows through the
serialized Dolt engine mutex; a fresh mint's `ReplaceLifecycleActivation`
waits behind that backlog until its 120s commit context dies, the submit
returns an error, and no desk cell is ever created. The trajectory stayed
live with zero desk runs — the same silent shape as the dead letter.

The fix retries the activation commit on `context.DeadlineExceeded` with a
fresh `WithoutCancel` deadline (`internal/agentcore/runtime_persistence.go`,
commit `ebfd2e98`). The commit is idempotent via its `CommandDigest`, so a
re-attempted mint cannot double-submit. Verified on staging: submit
`ffc400c2` minted and ran cell `e9bdff26`, which authored a turn on the
coordination doc — where before the mint error'd and no cell existed.

The full Ask->control->bind->deliver chain then proved out end to end on
staging: the cell's turn reason read "atomically open a research desk to
confirm that date," it minted `research:8764897c`, and control `1ed83383`
queued and delivered. Cancelling that carrier released the claim back to
pending — the stranded-bound release fix works.

The remaining residual is a substrate defect, not a steering gap: when the
freed control tried to re-bind, the persistent-Management desk was already in
a live-occurrence storm — a thundering-herd drain of post-boot obligations
fired en masse through the serialized Dolt engine, starving reads (staging
API 502'd) and queuing the rebind behind hundreds of other wakes. This is the
third live-lock family in the Management reconcile/redrive path; the fix is a
convergence invariant on re-drive issuance vs. drain rate, a substrate repair
named as the next boundary in
docs/problems/s0m-management-live-occurrence-storm-2026-10-03.md.

Receipts: fix `ebfd2e98` (deployed, guest refreshed); verification doc
`d138ff54`; storm doc + clustering assessment `f3959a12`/`d07bcb4e`; problem
docs docs/problems/s0m-postboot-deskmint-dispatch-starvation-2026-10-02.md and
docs/problems/s0m-management-live-occurrence-storm-2026-10-03.md; evidence
docs/evidence/s0m-stranded-bound-2026-10-03.json; trajectory
ffc400c2-d70e-58b3-b00c-c2a0917af6b9, cell run e9bdff26, research carrier
d8a17dee.
