# Debug Station M0 — The Restart That Ate Its Own Wake

September 30, 2026. This letter covers the debug and stabilize station, M0 — the first station of the supervision metamission — which wrapped in the small hours of this morning after about a day of work spread across two sessions. It also covers what I found when I tried to prove the next two missions on the live computer right afterward.

The news in one sentence: the owner computer is now healthy on the newest build, but proving the next missions exposed a single substrate bug — restarts and repeat inputs lose armed work — that now defines the next station's job.

Here is where things stand. Your computer is running the current build, its pending-change debt is reconciled to zero, and the screen-reconnect fix is verified live. Two of the three new capability missions are code-complete and sitting on the tape: typed commitment records and the Jev decision transport. What neither has yet is its deployed acceptance proof, and both proofs are blocked by the same defect. That defect is now documented, clustered, and folded into the signal-plane station that runs next.

## What the station was for

M0's job was mundane on purpose. The computer had accumulated a wedge of unreconciled desk state across earlier restarts, and the agreement was to redeploy, clean the debt, and re-run the quality-assurance reproduction before any new capability work continued.

The redeploy landed. The computer rebooted onto the new build at about four twenty-one in the morning, epoch nine hundred fifty-eight. Three stale pending runs reconciled to passivated, the desk's pending-mutation counter went quiet, and the reconnect fix I had been chasing — texture documents rejoining their event stream after a drop — passed its spec on the live build.

Then I ran the QA reproduction. It timed out. And the timeout turned out to be the most valuable thing in the whole run.

## Three failures that were one bug

The QA repro stalled on a texture leg: a document prompt opened the conductor, the conductor routed to the texture desk, and the texture activation passivated with zero tokens and no result. Digging into the run record showed the reason field as "runtime restarted" — a mid-run process restart killed the activation, and nothing ever woke the desk again.

I read the substrate and the mechanism is clear now. When the runtime restarts, it marks interrupted runs as passivated and marks their pending document mutation stale. The actor-wake outbox projector is supposed to be the sole post-restart delivery path. But the wake that fired the killed activation was already consumed, so the projector disposes it as a dead wake — and nothing re-mints the wake for the still-open obligation. There is a re-dispatch path in the texture owner, but it only runs at a full process boot, not on the runtime's internal restart path. The obligation stays armed; the deliverable is already gone.

While trying to prove the typed-commitment mission, the same pattern surfaced twice more, on different desks. An engineering cast that was in-flight across a restart got cancelled rather than replayed — "restart revoked absent assignment capsule" — because its executor capsule was gone and nothing re-drove the obligation. And when I revised a desk-bound document a second time while a cast was still open, the revise opened a duplicate work item instead of joining the live one, wedging the document at a conflict.

Three symptoms, one cause: the system mints delivery obligations at commit time, but after a discontinuity — a restart, or a second owner input while work is open — nothing reconciles the set of armed obligations against the set of live deliverables. Per the repo's clustering rule, I stopped patching and wrote the assessment: this is one substrate gap, and the durable fix is a single re-drive pass over armed obligations at runtime start, plus dedup of desk work items against open casts.

## What is proven and what is not

The typed-commitment schema is landed — precommit, resolve, and disagreement as typed cell verbs, with the string-commitment grandfathering rule and the acting-pack isolation all tested at the unit level. The Jev transport is landed and deployed — the gateway route calls OpenRouter's pinned model, the per-VM bearer is enforced, and my refusal probes showed missing and invalid bearers both rejected with a four-oh-one.

What is not proven is either of the deployed acceptance sequences. The commitment proof needs a desk to actually run a cell; the desk never fired — the same wake gap. The Jev round-trip needs a call from inside the guest; I could not reproduce the guest's network identity from the host without tripping the per-VM binding, and cells are sandboxed away from raw HTTP. Both proofs are waiting on the same repair.

One confession worth recording: during probing, I bound the owner VM's Jev bearer to the host's loopback address. The binding is durable, so I edited the on-disk registry and restarted the gateway to clear it from memory. Verified clean afterward. No lasting harm, but it was self-inflicted.

## What I'd do next

The next station, M-SUB, now carries the repair as its first real work item: re-drive armed obligations when the runtime starts, not only at boot, and dedup desk work items against open casts. Once that lands, the two queued deployed proofs should run unmodified — and that is the honest test of whether the cluster read was right.

## Names and receipts

M0 goal file: docs/definitions/choir-signal-m0-debug-stabilize-2026-09-29.md — status complete with residual.

Residual evidence: docs/evidence/m0-residual-texture-runtime-restart-2026-09-30.md.

Cluster assessment: docs/reviews/restart-lifecycle-rearm-cluster-2026-09-30.md.

Deployed build on the owner computer: commit 4c279162. Owner computer epoch 958. Today's commits: 0682b373, e3b4aa35, 45780777, 373d6002.

M1 landed in commit 5205cb5f. M4 landed in commit 4c279162. Both await deployed acceptance, gated by the restart-rearm repair in M-SUB.
