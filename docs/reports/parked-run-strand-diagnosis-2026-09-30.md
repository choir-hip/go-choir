# The supervision seam holds; one parked run still won't wake — September 30, 2026

Dateline: written Tuesday, September thirtieth, covering about eight hours of
work on the Jev-supervision metamission — the run that carries the desk-to-desk
supervision proof for Choir.

## The one-sentence news

The mechanism that was supposed to unstrand a stalled research run is now
deployed and proven live on staging — and the run is still stalled, which means
there is one more defect in the chain, now diagnosed and documented.

## Where the work stands

The metamission's job is to prove the four desks talk to each other over a
durable async signal plane: research emits evidence mid-cell, texture revises a
document, management scores the commitments. Most of that is built and landed.
Today the work was chasing one stranded research run — the one that everything
else is waiting on.

The run in question belongs to a researcher that was parked waiting for a
coagent's result. Its computer rebooted several times under a memory-pressure
kill on the host. Each reboot was supposed to wake it back up through a repair
I shipped earlier today — a chain of five commits that re-arms stranded wakes
and re-drives them with a fresh identity so they are not silently dropped.

That repair works. I can see it working: the guest booted, minted seventeen
hundred and seventy-eight pending wakes, and salted re-drive rows are landing
on the durable tape and being handled. Two of the environment faults I was
fighting earlier also cleared — the model provider circuit closed and is
streaming successfully again, and a flood of deferred work items poisoned
itself out cleanly instead of live-locking the guest.

## What I fixed and verified

Two of the three deliverables in front of me are done.

The commitment-ledger inspection items — proving the acting desk's commitment
pack carries no scoring fields, and that old pre-typed records stay readable
but unscoreable — are both verified. The acting pack is score-free at the type
level, not just by convention: the cell-execution function literally cannot
accept the score-carrying variant, and a pinned test marshals a real pack and
asserts no score keys ever serialize. That went into an evidence file and got
committed.

I also pushed the whole deployment loop to green: commits pushed, staging
serving the right build, the guest on the deployed epoch.

## What is still stuck

The research run is still passivated. That is now a diagnosed defect, not a
wait. I ruled out every simple explanation — it is not a queue backlog, not a
suppressed wake, not a dedup collision, not a write race. The wake re-arms
correctly and the salted row reaches the actor's mailbox. The failure is one
layer up: when the run's saved memory snapshot loses the pointer back to
itself, the handler takes a recovery path that consumes the obligation without
ever waking the parked run. The deliverable gets silently dropped — the worst
kind of bug, because it looks like success.

I wrote that up as a formal problem receipt before touching any fix, which is
the project's rule for a behavior change. The repair itself is a small,
well-understood seam: reactivate the parked run using the run id already
carried by the obligation, rather than only from the snapshot. It is flagged
as a protected change and I have a consensus panel reviewing the approach
before I commit to it.

## What this means

This is the fourth or fifth defect to surface in the wake-and-redrive
subsystem in about a week. Individually each is small. Together they say the
substrate has a pattern, and the project's own guidance is to stop patching
symptoms and look at the layer. The next mission is scoped to exactly this one
defect — not a rewrite — but it should land alongside a structural look at why
these strands keep appearing.

The metamission is not done. Two station proofs are blocked on this run
waking up. But the thing blocking them is now named, bounded, and has a repair
path under review.

## Names and receipts

Metamission goal file: docs/definitions/choir-jev-supervision-metamission-2026-09-29.md.
Problem receipt: docs/problems/coagent-result-parked-run-not-reactivated-2026-09-30.md.
M1 evidence: docs/evidence/m1-actingpack-legacy-grandfather-2026-09-30.md.
Next-mission draft: docs/definitions/choir-parked-run-reactivation-2026-09-30.md.
Head commit: ad362c22. Deployed staging build: 37882e1d. Stranded run:
362febb2, agent research:cfa90b87, trajectory ba6199f1. Redrive chain commits:
a2b87d69, a7e31232, 08a76896, 766b3a53, 37882e1d.
