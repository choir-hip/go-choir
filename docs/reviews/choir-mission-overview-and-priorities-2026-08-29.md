# Choir Mission Overview & Priorities — 2026-08-29

*Prepared for the owner after a 10-hour pause. Prose-first, zoomed out: where the
product stands, what happened while you were away, what is actually blocking
self-development, and the order in which to spend the next working sessions.*


## 1. The one-paragraph answer

The computer survived the night — and that itself is new. After two days of a
whole-system boot outage, staging booted cleanly at 21:54 UTC last night and
stayed up on a single generation, for the first time in 48 hours. But it did no
mission work overnight, and the reason is precise: the self-development engine's
next step — the persistent Super run that carries the nine queued cancellation
reports from the August 19 storm — was correctly dispatched at boot and then sat
in `pending` for 13+ hours. Nothing transitioned it from "dispatched" to
"executing." So the substrate war of August 27–28 is won, the self-development
flow is wired, and the last mile — dispatched-run execution — is the one broken
link between "healthy computer" and "self-development actually running."

## 2. What happened while you were away

An honest timeline of the overnight window (2026-08-28 22:04Z → 2026-08-29 11:30Z):

- **Nothing new ran.** Zero runs were created after 22:04Z. No candidate work,
  no Super execution, no capsule activity.
- **The texture residue run** (`ba204d8c`, resumed from the crash-loop residue)
  went `running` at 21:55Z and passivated at 22:00Z. This is very likely
  *normal* actor behavior, not a failure — Choir actors never "complete," they
  passivate on quiescence after an idle timeout. It was parked, not killed.
- **The persistent Super** (`fe92ea2b`) — dispatched by the boot rewarm at
  21:54:34Z with the assignment cancel-report bound as its recovery occurrence —
  remains `pending` with its update timestamp frozen at dispatch time. It never
  executed.
- **The self-development operation record** (`selfdev-ccf0f1ec…`) still reads
  `executing`, frozen since 2026-08-19T17:45Z — a zombie receipt from the original
  storm-era op that no state transition has ever closed.
- **A background loop did cycle all night**: a file-root citation retry walker,
  one attempt every 15 minutes, each deferring with an "invalid computer event"
  error. Non-fatal, undiagnosed, and a good example of background machinery that
  runs forever without paying anything — noted for later.

**Interpretation.** The computer is *healthy but dormant*. Health is no longer
the problem; *sustained self-directed execution* is. That is the correct next
frontier, and it aligns exactly with your stated arc.

## 3. Where self-development stands

The mission authority is now the compact candidate-proof Definition
(`docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md`), which
superseded the August 11 effects definition on 2026-08-20. Its success shape,
unchanged: **CoSuper authors candidate A inside its capsule** (a solitaire API
change with a pre-declared foundation defect), **freezes five verifiable
bundle references**, passes a **qualified reversible-selfdev-v1 consensus**,
**promotes A**, **plays it live** (API + durable rows + history), **falsifies
A**, **supersedes with B**, and **restores the computer to the pre-A fence**
(checkpoint `99949fe2`) — with every consequential transition recorded in
Texture. Levels: E1 (single promotion) is the rehearsal gate; E2 (the full
correction spine) is the vision. **None of E1 is paid yet.** Everything so far —
all 36 commits since August 28 — was substrate consumed *by* that proof, not the
proof itself.

The immediate blocked step is concrete: the nine cancel reports from the August
19 storm are still queued undelivered; the fresh CoSuper binding that would
process them never happens because the Super that carries them never executes.

## 4. The priority ladder

Your framing is right, and the evidence supports it. In order:

1. **Self-development working via CLI.** This is the gate for everything else.
   Concretely: fix the dispatched→executing gap, let the Super consume the nine
   cancel reports, bind a fresh CoSuper, and run the candidate A proof chain
   end-to-end. Until this is done, nothing downstream is real.
2. **Autoputer standalone.** A computer that stays *self-sustaining* — boots,
   resumes its own work, keeps its own machinery moving without an operator
   (or a coding agent) nudging it. Last night showed exactly the gap: healthy
   boot, zero self-initiated work. "Standalone" is proven the day the computer
   resumes its own mission after a restart with no external wake.
3. **Autopaper (World Wire).** Only after a genuine self-developing computer
   exists; the doctrine is explicit that the wire rides the same
   tape/artifact/settlement spine and is not built ahead of the computer.
4. **UI/UX — your instinct is correct, it is not the priority now.** But one
   nuance matters for planning: when you return to it, the target is *self-dev
   through the prompt bar and Texture*, not a general polish pass. The July UX
   review's five findings are largely already repaired in source (verified by
   reading the current Svelte code: keyboard shortcut normalization, toast
   dismissal, focus guards, theme apply/revert) but were never verified on
   staging — so the eventual UX milestone is "drive a real self-dev cycle from
   the Choir UI," which will surface the *relevant* bugs naturally, rather than
   fixing a stale list blind.

## 5. The pressing-issue queue, ranked

**P0 — Dispatched-but-never-executing Super.** The rewarm dispatches runs; the
actor loop never picks this one up. `fe92ea2b` is a durable queued activation,
not an execution. Small, well-scoped, and the single blocker for candidate A.
This is the next fix, and per the house rule it gets a problem receipt first.

**P1 — Substrate residuals that threaten the flow (receipted, unpaid):**

- *Object-graph body scans*: the August 28 root-cause clustering receipt proved
  one common cause behind seven crash-loop symptoms and landed the per-field
  index for the Super-rewarm path only (`42d47604`). The remaining
  worker-update/sweep callsites are still unwired; each is a standing
  performance cliff that previously killed the boot.
- *Mailbox backlog*: 1,280 delivered-pending worker-update packets and growing
  tombstones inflate every scan that touches them.
- *Unbounded tape*: the computer's event chain exceeds 133,000 events with no
  pruning policy; every full replay/scan pays it.
- *Persistent-image disk headroom*: the host VM directory sits near ~98 GiB
  against a 32 GiB virtual cap; when the host disk fills, the guest dies. This
  is a physical ticking clock, not a code bug.

**P2 — Product-surface problems, documented but unrepaired:** the
single-health-fail degraded flip (one probe flips the computer to degraded);
account recovery for `yusefnathanson` (problem receipted 08-27; the repair
route — Track M — is not wired); and the zombie `executing` self-dev operation
record that should be closed out when the wake path next fires.

**P3 — UX verification and polish.** After self-dev works. See §4.

GitHub, for completeness: there are **zero open GitHub Issues** — the search
API returns an empty set for open and closed issues alike. Two items that look
like issues (#52 ChatGPT web-search provider, #44 model-policy helper
extraction) are actually open pull requests. The UX backlog lives in docs
(`docs/ui-ux-review-2026-07-10.md`, plus the auth onboarding review), not in
the tracker.

## 6. What "self-development working" means (the finish line, concretely)

When you next sit down to review, these are the observable facts that will
demonstrate the mission — no trust required, all product-path:

1. A Super run leaves `pending`, executes, and *consumes* the nine cancel
   reports (delivery recorded; no restorm).
2. A fresh CoSuper assignment binds; its capsule boots on staging.
3. Candidate A is authored, built, tested, frozen (five bundle refs), and
   proposed — inside the capsule, not by hand.
4. A qualified consensus panel approves the exact subject.
5. A is promoted; the play path writes API-visible rows durably.
6. A is falsified; B supersedes it; the computer restores to checkpoint
   `99949fe2` and the API/rows are absent again, with history retained.
7. Every step above is recorded in Texture with digests and receipts.

Effects (the actual email/send machinery) stay OFF the whole time — this is the
rehearsal envelope, by design.

## 7. Recommended next 48 hours

1. **Problem receipt + fix for the dispatched-pending Super gap** (P0), then a
   live proof: `fe92ea2b` (or its successor) executes, consumes the cancel
   reports, and binds the fresh CoSuper.
2. **Run the candidate A chain** while the computer is healthy — it is the
   mission's core and the only thing that pays E1.
3. **Wire the remaining body-scan callsites to the field index** (the
   clustering receipt's named next action) so the next boot outage cannot
   recycle through the same family.
4. **Disk headroom check on Node B** — five minutes of operator attention that
   removes a physical single-point-of-failure.
5. Defer UX and the smaller P2s until the chain in §6 is paid.

## Appendix — Receipt map (last 48 hours)

| Where | What |
|---|---|
| `docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md` | Current mission authority (supersedes the 08-11 effects definition) |
| `docs/problems/` (12 files, 08-28) | Boot crash-loop family, image-full warning, single-health flip, rewarm scan chain, worker-update scan, selfdev wake no-op, texture boot ambiguity |
| `docs/evidence/root-cause-clustering-objectgraph-body-scan-2026-08-28.md` | Seven symptoms → one og_objects body-scan cause; per-field index partially wired |
| `docs/evidence/account-recovery-yusefnathanson-2026-08-27.md` | Recovery problem documented; repair (Track M) not landed |
| `docs/reviews/passivated-authority-structural-assessment-2026-08-28.md` | Consensus 11/11 → B′ contract; landed as `0ec9c2f0` |
| `0ec9c2f0` / `6db76078` | Deterministic passivated-residue supersession + live boot-liveness proof |
| `ca01210f` / `42d47604` | Self-dev wake revive; canonical field-index substrate |
| Fence | Pre-A checkpoint `99949fe2`, epoch 837, effects OFF throughout |
