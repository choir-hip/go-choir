# S0m Closes — Desks Now Speak in Records

Written October fourth, 2026. This letter closes the S0m station of the
supervised app development metamission — three days of work to make every
desk-to-desk act a typed commitment record — and it replaces the October
second checkpoint letter as the thing to read. An earlier letter covered
the first two days in detail; this one resolves the open questions it left.

## The short version

S0m is done, and the proof ran live on staging: the texture desk asked a
researcher for a date, the researcher answered, the answer came back bound
to its activation, and the desk applied it and settled the work — all of
it riding on commitment records instead of raw messages. The machinery
held. The surprises were elsewhere: a security finding large enough to
reorder the mission, and a deploy pipeline that cancels itself.

## How the last two days went

The checkpoint letter left three threads open. The stranded-packet
deadlock had been fixed but ten old packets were still wedged; the ask
that was supposed to close itself never did; and a substrate storm was
starving the Management desk's commits.

The resolution turned out to be two small substrate repairs and one
model-agency finding. A carrier run that finishes now folds its bound
control into its own report commit as incorporated — the packet is counted
consumed by the thing that carried it, so nothing dangles. A mask in the
idle-trigger path was reading a non-apply cell as "no act"; it now records
the honest decision kind, so a desk that chose not to edit is no longer
indistinguishable from a desk that never ran. And a path-parity audit
came back clean: no unmarked bound-control write path remains.

With those in place the acceptance leg ran end to end on staging: ask,
research, report, consume, apply, settle, all recorded on the commitment
ledger on one trajectory. The stranded-packet ten from before stay
unresolved — they predated the release fix and their carriers are gone —
and that is now a named residual rather than a mystery.

The remaining desk-agency gap is real but bounded: the desk can mint a
resolve, the verb exists, and when the model takes the step the chain
completes mechanically. Whether closing on consume should be automatic is
a design question, not a defect, and it stays open.

## What the station found on the way through

The work discovered more than it fixed. The largest: from inside any
guest, a caller can reach the host's internal services and assert
internal authority with a header — which, traced through source, amounts
to lifecycle control over every computer and cross-tenant reads. That
finding is now the next station's work, pulled ahead of the remaining
measurement probes because the deployment allows open registration. The
station also surfaced a deploy pipeline that cancels an in-flight deploy
when a documentation push lands, a guest-runtime deploy gap for
constructed computers, and a disk-headroom squeeze on the deploy host
that was cleared by deleting an old rollback image. One more surfaced at
the boundary itself: a freshly registered computer reports active before
its canonical event chain has a genesis, so its first write fails until
the repair route runs — filed for the boot-timeline station.

One honest heresy to own: the texture desk model was changed with a
manual live edit inside the guest's data disk — untracked state drift.
It worked, it is recorded, and the layering station is where that kind
of change becomes a release instead of a hand edit.

## What comes next

The next slice is the host-boundary hotfix: per-tap network isolation,
real authority binding in place of caller-asserted headers, and a
deployed refusal matrix proven on disposable accounts. The deploy-pipeline
fix already landed in passing — a guard that refuses to deploy a commit
stale relative to what is live — alongside the boundary-close proofs:
the stranded-packet recovery re-run on a real disposable computer this
time, and channel mail to a texture desk refused at cast time on staging.
After S1a, the measurement probes resume.

## The one thing to remember

The failure class S0m retired was silence: a message that looked sent
and never arrived, a bound packet nobody could see, a desk run that
starved before it existed. The repair pattern that finally held was to
make every state transition leave a record an observer can score — and
then to fix the places where the record itself lied about what happened.

## Names and receipts

Station goal file:
docs/definitions/choir-appdev-s0m-record-native-messaging-2026-10-01.md.
Deployed terminal commit: a4fcdb8d, deployed under CI run 37165170516
(confirmed by x-choir-build-commit). Boundary-close receipts:
docs/evidence/s0m-ask-acceptance-2026-10-04.json (trajectory 684ddcb1),
docs/evidence/s0m-stranded-bound-disposable-2026-10-04.json
(disposable computer-ca3a2cf9: cancel -> claim released -> rebound ->
incorporated),
docs/evidence/s0m-channel-mail-reject-2026-10-04.json
(trajectory aef9a197: choir.Emit to "texture" refused at cast time).
Consume-marking fix: d61c9b1b; idle-mask fix: d20483e8; channel-mail
reject: b18f3baf; stranded-bound release: c9180cd3; mint-retry fix:
ebfd2e98.
Problem docs opened by the station:
docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md,
docs/problems/s0m-management-live-occurrence-storm-2026-10-03.md,
docs/problems/s0m-guest-runtime-deploy-gap-2026-10-02.md,
docs/problems/node-b-deploy-disk-headroom-2026-10-04.md,
docs/problems/s0b-registration-computer-missing-genesis-2026-10-04.md.
Texture model drift record:
docs/evidence/s0m-texture-model-swap-2026-10-02.md.
