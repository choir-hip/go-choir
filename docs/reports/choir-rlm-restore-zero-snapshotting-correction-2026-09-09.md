# Restore-Zero Was Not Snapshotting. Here Is What That Meant, and What Changed.

**Date:** 2026-09-09
**Subject:** Why `yusefnathanson@me.com` replayed a lifetime of events, why the earlier "complete" claim was false, and what the snapshotting panel actually required.

This is not a completion report. The earlier Restore-Zero completion PDF should be read as a substrate-progress note that overclaimed. The invariant that finishes this mission is: **after a verified snapshot W exists, no recovery path may replay the prefix.** That invariant was not true on the live owner computer. It is now encoded in boot, and the false skip is gone. Staging still has to *publish* a watermark near the live head before that computer can boot from a snapshot instead of refusing.

## What you actually saw

The account did not boot. The browser got `{"error":"failed to resolve user autoputer"}`. Behind that JSON, the guest was not "stuck resolving." It was reconstructing the computer from a stale local head, tens of thousands of events behind the platform, while vmctl held a refresh lock and the proxy had nothing to paint except a 502.

That long reconstruct is the failure. The 502 is a symptom. Restore-Zero exists so that reconstruct never looks like a lifetime of tape.

## Why the huge tail happened

Choir already had snapshot machinery. Offline `choir-rebuild-base` can rebuild a ProjectionBase blob. The platform can advertise a watermark W. Rematerialize already knew how to install that blob into a sibling directory, replay only `(W, H]`, and swap. Boot did not use that path.

Boot asked one question: *is the store directory empty?* If yes, install a base. If no, skip and resume. The owner computer's disk was not empty. It had a local head around 20,000. The platform head was around 148,000. The advertised watermark was 1 (a 13-blob also existed and was never even the advertised W). Because the directory contained files, boot treated "resume" as recovery. Resume from 20,000 to 148,000 is a 128,000-event lifetime catch-up. The snapshots sat on the platform unused.

That skip lived in `materializeProjectionBaseIfNeeded`. Two tests treated it as correct behavior: a non-empty store was supposed to cause *zero* platform reads. Those tests encoded the defect.

So Restore-Zero, as deployed, snapshotted **empty** computers. It did not keep W close to H. It did not rebase a retained computer onto W. Publication of W=1 / W=13 plus an HTTP 200 was never proof that recovery had become tail-only.

## What the second panel decided

The first consensus panel spent its energy on login 502s. That was the wrong question. A second panel, asked only about snapshotting, converged on a contract that matches the mission:

1. **Who writes W.** Host-side `choir-rebuild-base` remains the publisher. After a successful rebuild it must POST the watermark; today the CLI printed JSON and left the table at W=1 while a W=13 blob already existed. Guest reconstruct does not become the watermark writer. A periodic host rebuild is the cadence that keeps W near H. Publishing on every refresh as the *primary* writer was rejected: deploy SHA is not H, and it couples snapshot cost to boot.

2. **When a non-empty store must consume W.** Not "never" (current). Not "only if the gap is large" as the rebase trigger. Always rebase when `local < W`. Resume only when the retained head already descends from W *and* the remaining tail fits a named bound. If advertised W is itself stale, **refuse** rather than replay 128k events. The bound is `MaxRecoveryTailEvents = 10,000`. Today's shape (local ≈ 20k, W = 1 or 13, H ≈ 148k) must fail closed.

3. **How to install W.** Reuse the rematerialize staged install: sibling directory, `InstallVerifiedBase` (empty dest), quarantine the original marker and texture workspace, flip, never overwrite live SQLite in place.

4. **The test that would have blocked the false complete.** Retained local=20,000, advertised W=148,000, frozen H=148,333. Recovery may apply 333 tail events and zero prefix events. The same fixture with W=13 must refuse with zero replay. Guest `/health` JSON is not that test.

## What I implemented

The decision is a pure function, `PlanRecovery`, so it can be tested without opening Dolt:

- empty + no chain → genesis (the only genesis)
- non-empty + no chain → refuse
- chain + missing or stale W → refuse
- empty + fresh W → install in place
- local < W + fresh W → rebase
- local ≥ W + tail inside 10,000 → resume

Boot calls that plan on every credentialed startup, including retained disks. The empty-store skip is deleted. A retained computer whose remaining work would exceed 10,000 events dies with `projection base refused` instead of disappearing into a 128k reconstruct. A retained computer that is behind a *fresh* W is rebased: the verified blob is unpacked beside the live store, the original is quarantined, the new marker and `.texture` workspace are flipped into place, then reconstruct starts at W.

`choir-rebuild-base --advertise` now POSTs the published blob through the existing watermark handler, which remains the sole writer of `computer_replay_watermarks`. Publishing a blob without advertising it is how W=13 existed while the table still said W=1.

Rematerialize uses the same tail bound: a restore whose `(W, H]` exceeds 10,000 refuses rather than becoming another lifetime replay.

Local tests that now exist, and that the previous completion could not have passed:

- PlanRecovery on the live staging numbers: W=1 and W=13 against H=148,333 refuse; local=20,000 with W=148,000 rebases a 333-event tail; local already past W resumes.
- A retained store installed at W=1, then rebased onto W=2, opens at sequence 2. The original realization is quarantined. Staging directories do not leak.
- Boot materialize against W=13 / H=148,333 on a real retained disk refuses and leaves the marker in place.
- Advertise POSTs computer, sequence, and blob digest to the watermark endpoint.

## What is still not done

This computer still will not boot until a watermark near the live head is published. That is the point of fail-closed recovery. With W still at 1, boot now *refuses* instead of replaying 128k events. Refusal is the correct behavior. It is not owner-visible boot.

The remaining operator step is host-side publication, not another guest reconstruct:

```
choir-rebuild-base --computer <owner-computer-id> --target-head <current-H> --advertise
```

That rebuild is allowed to read the prefix. It is publication, on the host, into an isolated scratch store. It is not recovery. After it advertises W ≈ H, the next guest boot should rebase (or resume if local already passed W) and apply only `(W, H]`.

Until that boot is observed with prefix reads = 0 and tail = H−W, Restore-Zero is not complete. Do not treat this note, a passing local test, or a new SHA as deployed proof.

## Why the 502 still matters, and why it is not this patch

While a guest is reconstituting, resolve can fail and the proxy currently answers JSON. That is a routing/UX defect. It is real. It is not snapshotting. If we had "fixed" the 502 first, the owner would have watched a boot screen for the length of a lifetime replay. The snapshot contract is the reason that wait should be a few hundred events, not a hundred thousand.

## Short version

The mission is snapshotting so we never replay the whole log. We had snapshots that only attached to empty disks. The owner's disk was not empty, so boot ignored the snapshots and replayed history. The panel said: delete that skip, refuse a stale watermark, rebase a retained disk through the install path we already had, and make the publisher actually advertise W. I did that. The owner computer will boot from a snapshot when one exists near H. It does not exist yet. Publishing it is the next act, not another claim of completion.
