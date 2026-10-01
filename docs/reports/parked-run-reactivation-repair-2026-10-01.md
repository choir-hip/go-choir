# The parked run has its fix; the computer won't stay up long enough to prove it — October 1, 2026

Dateline: written overnight into October first, covering the landing of the
parked-run reactivation repair and the start of a broad owner QA drive on the
staging computer.

## The one-sentence news

The defect that let a parked research run lose its deliverable — a wake that
discharged without ever reactivating the run it was bound to — is now repaired,
deployed to staging, and pinned by a regression test. The only thing left is
watching it fire on the live guest, and that is waiting on the guest staying
alive long enough to drain its wake queue.

## What the repair does

Yesterday's diagnosis named a resume-authority gap. When a bound `coagent_result`
obligation reached a parked run whose actor snapshot had lost its resume
pointer, the handler fell into a generic reconcile path that marked the
obligation handled without ever reactivating the bound run. The deliverable
disappeared once, silently.

The fix makes the obligation itself authoritative. On that arm the handler now
resolves the exact persisted packet by its content digest and, when the packet
names a bound run, feeds that run id into the existing parked-run reactivation
branch. The same transaction and guards as a normal resume — so the snapshot
gets healed too. If the packet names no run, it falls through to generic
reconcile; if it names several, it defers rather than guess. The hard rule is
preserved: generic reconcile still never selects a run.

A nine-agent panel reviewed the seam beforehand and accepted it, with the
refinements that shaped the final code — content-digest resolution over the
actionable pending set, reuse of the parked branch's own guards, and the
correction that the live strand was more likely a silent incorporate than the
hard-error arm.

## Verified and deployed

The repair carries a regression test that reproduces the exact shape — a
passivated run, a bound pending result, an empty resume snapshot — and asserts
the run leaves passivated and the snapshot heals. The full actor-runtime
package passes under the race detector across all CI shards; the deploy landed
and staging now reports the new build commit.

The one open receipt is the deployed observation itself: watching the known
stranded run (`362febb2`, bound control `4158e48b`) actually reactivate when a
wake drains onto the fixed handler. A watcher is polling the guest journald for
that transition.

## The environment is the blocker

The staging guest is cycling on a host memory-pressure kill — the platform
database balloons and the VM is killed and recreated roughly every twenty to
forty minutes. Each boot re-arms the stranded wake (the wake-outbox migration
re-mints it), so the fix will fire on the first drain that outlasts a kill —
but the drain is serial and the windows are short. This is the named residual:
the code is proven, the deployed observation is capacity-gated.

That same instability surfaced a second, separate problem during the QA drive:
submitting a task to the computer intermittently returns `502 failed to resolve
user autoputer` even while the guest is internally alive and computing. The
routing layer refuses a submission whenever the computer's realized state is
mid-transition — which, on an OOM-cycling guest, is a large fraction of the
time. That is a routing-plane availability gap, distinct from the wake defect,
and it is documented in its own problem receipt.

## The QA drive

The owner's overnight ask was to exercise the computer end-to-end: broad
current-events research plus self-development proposals for the slideshow,
ebook/PDF reader, and podcast apps. The research mechanism proved real — each
prompt opens a document, spawns a research desk, produces bound coagent
packets, and the desks write revisions back. Early submissions landed; later
ones queued behind the routing flap. A detached driver is retrying the full
nine-topic spread through the down windows.

## Where it lands

The repair is done and its proof is a matter of a stable uptime window, not
more code. The two substrate findings — the OOM cycle and the routing flap —
are now documented for the missions that own them. When the guest holds, the
watcher captures the reactivation, the research findings land in their
documents, and the report closes.
