# Problem Receipt: Boot-Resumed Persistent Super Run Hangs at Pending and Blocks the Singleton Slot

- Date: 2026-09-03
- Mutation class of this receipt: green (documentation); the repair it directs is red
- Status: documented before any code fix (problem-documentation-first)
- Discovered by: deployed acceptance for Definition 1 criteria 1/3 on staging `9b348aaf`

## The Problem

The structurally isolated boot resume (Definition 1 repair, working as
designed at the controller layer) reactivated exact interrupted run
`fe92ea2b` on staging boot epoch 850:

```text
05:10:19 boot persistent-Super rewarm candidate run=fe92ea2b ...
05:10:19 persistent-Super rewarm packets run=fe92ea2b pending=1
05:10:19 persistent-Super exact-run resume reactivated run=fe92ea2b ...
05:11:04 actorruntime: persistent Super recovery received agent=super:5bd6de97-...
```

The run then never progressed: it has stayed `state=pending` since
reactivation with no execution, failure, or completion log line. Because a
pending run is an active resident of the I26 singleton slot, every live
Texture trigger wake during this window hit the resident-bind branch in
`wakeUpdatedCoagent` (`bind resident lifecycle controls target=super:...
<nil>` at 05:16:58), matched none of the resident's work items, and returned
without minting — the queued ordinalized execution_requests (ordinals 2, 3,
4) stayed pending. A hung resident therefore blocks all live-trigger
scheduling indefinitely.

The resumed run's single delivered packet is storm-era residue
(`assignment-report:cancel-report:sha256:ca295d0c...`, delivered to the run
during the 2026-08-19 storm). 52 further passivated `runtime_restarted`
control runs with delivered packets remain as rewarm candidates, so each
boot will resume the next stale candidate and can reproduce the hang.

## Root Cause Clustering

Same substrate as the boot-as-scheduler heresy: storm-era residue
(delivered cancel reports bound to stale interrupted runs) is still live
state that the recovery path keeps trying to execute. Criterion 4 settled
the pending-and-undelivered half; the delivered-to-stale-run half remains
and (a) gives every boot a resume candidate, (b) makes the resumed run's
first activation a stale-cancel-report model loop, and (c) observed here,
hangs at pending inside the actor activation.

## Candidate Repairs (owner decision required)

1. **Settle delivered stale residue**: extend store-layer settlement to
   delivered cancel reports bound to terminal/pending stale runs (CAS on
   delivered_to_run_id + disposition pending), so the resume selector finds
   no pending delivered packets and boot rewarm becomes a no-op. This
   widens criterion 4's CAS letter ("pending and undelivered") and needs
   owner ratification.
2. **Bound the resume activation**: give the recovery occurrence's
   activation a bounded execution deadline that terminalizes the run on
   hang, so a hung resident can never block the slot indefinitely.

Interim owner correction used for acceptance (I26-permitted): cancel the
hung run via the product path; its terminal state removes it from the
resume-eligibility filter.
