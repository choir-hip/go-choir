# The Promotion Protocol: Mutation/Transaction Conjecture — 2026-06-11

## Status

Conjecture-program artifact for the mutation/transaction system, produced from
four research passes (internal code audit; saga/2PC/Percolator transaction
theory; A/B-OS/blue-green/Dolt deployment prior art; change-review UX prior
art) plus the new spec `specs/promotion_protocol.tla` (model-checked green;
three sabotage variants caught, **two of which encode today's actual code
behavior**).

Companions: `docs/choir-rearchitecture-durable-actors-2026-06-11.md`,
`docs/handoff-hybrid-computer-capsule-architecture-2026-06-10.md` (the
original MutationTransaction sketch), `specs/README.md`.

---

## 0. The honest current state (code audit findings)

The promotion system today is thinner than the docs imply:

1. **"Promotion" is a database pointer flip with no deploy.**
   `PromoteAppAdoption` (internal/runtime/app_promotion.go:407-453) updates
   `ComputerSourceLineageRecord` fields (`ActiveSourceRef`, digests,
   `RouteProfile`) — no real git ref move, no route switch consumed by any
   proxy, no process restart, no binary swap. The candidate build output sits
   in a scratch workspace, never wired to anything live. The UI says
   "Activated"; the running computer is unchanged. (Accidental safety from
   incompleteness, not designed safety.)
2. **Rollback restores 4 strings.** `RollbackProfileJSON` mirrors four
   lineage fields, no TTL, no snapshot/generation/Dolt/route refs. It is a
   pointer rollback of a pointer promotion.
3. **The owner-approval gate is dead code.** `owner_approved` exists as a
   status, is accepted by `PromoteAppAdoption` — and nothing in the system
   ever produces it. No approve tool, endpoint, or button. Promote fires
   straight from `verified`.
4. **Foreground divergence is recorded, never enforced.**
   `TargetActiveSourceRefAtCandidateStart/AtCutover`,
   `ForegroundTailMergeResult` are stored; no check blocks promoting a stale
   candidate over newer foreground state — exactly the failure the
   legacy-promotion learnings doc forbids.
5. **The curl|bash guard defaults off.** `guardForegroundSuperMutation`
   (tools_coding.go:568-579) is a no-op unless
   `RUNTIME_SUPER_FOREGROUND_MUTATION_MODE=worker_only` is set; even then it
   only constrains foreground super. Real protection awaits capsules.
6. **The changes app is `FeaturesApp.svelte` ("Features"), and the docs
   don't know it.** `platform-os-app-state.md` still describes "Apps &
   Changes" with Uninstall/Disable/portfolio-review — none of which shipped.
   Features has Import/Activate/Rollback/Roll-forward only, shows hashes not
   diffs, hardcodes `TARGET_COMPUTER_ID='primary'`, and **never calls the
   preview endpoint that already exists**
   (`/api/adoptions/{id}/preview/*`, api_app_promotion.go:417-517).
7. **Zero Dolt/NixOS/VM-snapshot/Qdrant integration** — the
   MutationTransaction's other six ledgers have no implementation footprint.

What IS solid: `materializeAppAdoptionCandidate` (clone, patch-apply, build,
hash) — a real, working `source_build` prepare step; the
no-cross-computer-binary-copying verifier rule (recipients rebuild from
source); the secret-payload scan; the AppChangePackage as a typed, portable
change object.

---

## 1. The protocol design (what the spec encodes)

Research delivered a crisp shape. Five principles, each with named prior art:

### 1.1 Single commit point (Percolator / route pointer)

Do not make N heterogeneous ledgers commit simultaneously. Designate **one
tiny atomic flip** — the promotion record / route pointer — as the
linearization point AND the visibility gate. Everything else is a
"secondary" whose fate is decided by reading the commit point alone: any
reconciler (or crashed-and-restarted coordinator) can finish an interrupted
promotion by rolling secondaries forward (commit happened) or back (it
didn't). Users see entirely-old or entirely-new, never a mix, because the
route pointer is also what gates reads.

### 1.2 Per-ledger prepare/apply with 2PC-shaped states

Each ledger (source ref, Dolt data, derived index, blobs, NixOS generation,
VM snapshot) moves `none → prepared → applied | rolled_back`. Prepare is
durable, idempotent, and inert — built and verified under a private identity
(shadow Qdrant collection, candidate branch, inactive slot) before the flip
ever references it. The 2PC "Consistent" invariant generalizes: no ledger may
end `applied` while another ends `rolled_back` for the same promotion epoch
("torn rollback" / consistent-cut violation).

### 1.3 The pivot (saga theory) + try-then-confirm (Android A/B)

Commit is the point of no return. Before it: abort + compensation is always
safe. After it: forward recovery only — and a post-commit **health window**
ends in Confirm (mark slot good) or AutoRevert (the A/B try-counter pattern:
silence defaults to revert). A revert is a reverse promotion through the same
machinery, not a panic-undo.

### 1.4 The freshness CAS (Dolt three-way / foreground tail)

The active computer keeps living during candidacy. Promotion is therefore a
three-way problem (fork point, candidate changes, foreground changes since
fork). Commit requires the foreground tail to match what the candidate was
prepared and verified against; if it moved, **restage** — re-prepare against
the new base, with verification AND approval invalidated (evidence about a
stale base authorizes nothing). Structural conflicts hard-block; data
conflicts get explicit resolution, never silent last-writer-wins.

### 1.5 The rollback window and poisoned writes (blue-green N-1 rule)

Rollback is cheap pointer-re-flip **only while the old version can still
read everything the new version has written**. The first N-1-incompatible
write closes the window — after that, auto-revert is the torn-rollback bug,
and recovery must roll *forward* (corrective promotion). Consequences:
- destructive/contract changes (drop old schema) belong in a *later,
  separate* promotion after the health window confirms — never bundled with
  the expand;
- "is the rollback window open" is explicit durable state the UI can show;
- shared user data (the `/data`, `/var`, `/home` analog) is **outside** the
  atomic scheme entirely and is where all real risk concentrates — Dolt
  branching/merge is Choir's structural advantage here, but only if
  promotions actually create Dolt branches (today they don't).

### 1.6 What the spec checks (specs/promotion_protocol.tla, green)

| Invariant | Meaning | Sabotage result |
|---|---|---|
| SecondaryFollowsCommitPoint | secondaries never lead the outcome | — |
| NoTornOutcome | settled promotions are uniform across ledgers | — |
| **NoStaleCommit** | no commit against a moved foreground tail | **violated by today's code path** (no CAS in PromoteAppAdoption) |
| **ApprovalGate** | nothing becomes visible without owner approval of *this* staging | **violated by today's code path** (`owner_approved` dead, promote from `verified`) |
| RevertSafety | no revert after a poisoned write | violated when the window guard is dropped |
| CommitPointDeterminesOutcome (liveness) | no promotion hangs half-reconciled | — |

Two of the three sabotage variants are not hypotheticals — they are formal
certifications that **the current implementation violates its own intended
protocol** in two specific, fixable ways.

---

## 2. Relationship to AppAdoption: generalize, don't replace

`AppAdoption` is not discarded — it becomes the **source_build ledger's
prepare step** inside the MutationTransaction. The mapping:

| MutationTransaction concept | Existing artifact |
|---|---|
| prepare(source_build) | `materializeAppAdoptionCandidate` (works today) |
| verifier evidence | `VerifierResultsJSON` + contracts |
| prepare(dolt_app) | NEW: Dolt branch + three-way merge plan |
| prepare(derived_index) | NEW: shadow collection + alias plan |
| commit point | NEW: promotion record + real route flip |
| rollback refs | `RollbackProfileJSON` → upgraded to per-ledger refs + TTL |
| owner approval | `owner_approved` → resurrected as a real gate |
| freshness CAS | `TargetActiveSourceRefAtCutover` → enforced, not recorded |

Capsules (hybrid handoff Milestones 1–3) slot in as where prepare-effects are
*captured*; they are not blockers for fixing the protocol semantics above.

---

## 3. The changes app (Features) — redesign direction

Research consensus: **preview beats diff for non-developers; plan-before-apply
is the approval artifact; rollback is a restore-point timeline, not a revert
of change #4821.** The core review loop, one screen, one decision:

1. **Headline** — one plain-English sentence, first ~100 chars do the work
   ("Fixes photos not syncing"). Generated summary; accuracy is a
   conflict-of-interest risk if the authoring agent writes its own — an
   independent summarizer (or deterministic plan-diff) should produce it.
2. **Try it now** — live preview of the candidate. The endpoint already
   exists and the UI never calls it; this is the single cheapest high-value
   fix in the whole system.
3. **Plan** — structured "will change / will NOT touch" list with
   destructive/irreversible items visually distinct (Terraform-plan model).
   Includes rollback-window status ("fully reversible" vs "this change
   cannot be undone after X").
4. **Check badges** — 2–4 plain-language gates ("data integrity verified",
   "no other apps affected"), blocking Approve until green.
5. **One decision** — Approve & Activate / Reject / Not now. Approval is the
   resurrected `owner_approved` transition; Activate is the commit point.
6. **Tiers** — low-risk packages default to "applies tonight unless you say
   no" (iOS pattern); high-risk waits for explicit review. Risk
   classification must not be self-assessed by the authoring agent alone.

Rollback UX: a **timeline of named restore points** ("Before: Photos update,
June 10"), one tap, instant-feeling; explicitly state what happens to data
created since promotion ("3 photos added since — they'll be kept / lost");
proactively offer restore on detected post-promotion crash signals (browser
session-restore pattern).

---

## 4. Conjecture ledger

### P1 — Single-commit-point promotion is implementable as a thin layer now
- **Claim:** a durable promotion record with the spec's state machine,
  wrapping existing AppAdoption as the source_build prepare, plus a *real*
  route flip, retires gaps 1–4 of §0 without waiting for capsules.
- **Test:** one end-to-end promotion on a real computer where "Activate"
  observably changes what the running computer serves, with the spec's
  guards enforced in code; kill the coordinator mid-promotion and verify a
  reconciler finishes it from the promotion record alone.
- **Edge:** the route flip needs a consumer (proxy/vmctl) that today ignores
  `RouteProfile`; if the flip stays cosmetic, the whole protocol is
  ceremony around a no-op. The flip consumer is the load-bearing unknown.

### P2 — Approval and freshness guards are one-day fixes with outsized value
- **Claim:** enforcing `base = tail` (CAS on `ActiveSourceRef`) and
  requiring a produced `owner_approved` before promote brings the
  implementation into conformance with two spec invariants immediately.
- **Test:** the two sabotage variants, inverted: after the fix, attempts to
  promote stale or unapproved adoptions are rejected; TLC-spec conformance
  by construction.
- **Edge:** an approval gate without the §3 review surface is a rubber stamp
  ("decorative consent"); ship the headline+plan+preview minimum with it.

### P3 — Dolt branching is the answer to the shared-state problem
- **Claim:** making promotions create real Dolt branches at fork time and
  three-way merge at commit converts the worst risk class (user data) from
  blue-green-style prayer into mergeable, conflict-surfacing state.
- **Test:** a promotion whose candidate migrates a table while the
  foreground writes rows; commit either cleanly merges or blocks with a
  legible conflict set — never silently drops either side.
- **Edge:** cell-level merge can auto-resolve things that are semantically
  conflicting; resolution *policy* (active-wins for live data,
  candidate-wins for config?) is an open design decision, not a default.

### P4 — The rollback window must be explicit state, surfaced in the UI
- **Claim:** tracking "window open/closed" per promotion (closed by first
  contract-phase or N-1-incompatible write) and gating both AutoRevert and
  the user's Rollback button on it prevents the entire torn-rollback class.
- **Test:** RevertSafety invariant in code-conformance tests; UI shows the
  window state; a contract-phase change is structurally forced into a
  separate later promotion.
- **Edge:** classifying writes as N-1-compatible is itself fallible —
  start conservative (any schema change closes the window) and loosen with
  evidence.

---

## 5. Founder questions (the ones not yet considered)

1. **What consumes the route flip?** Nothing reads `RouteProfile` today.
   Until proxy/vmctl honors it, "Activate" cannot be made real. Who owns
   this — vmctl route table, proxy resolution, or both?
2. **What is one "change package" from the user's view** — one intent/outcome
   (however many files), or one agent run? Determines whether one-decision
   review is achievable.
3. **Who writes the headline/plan** — the authoring agent (conflict of
   interest), an independent agent, or a deterministic diff-summarizer?
4. **Where is the auto-apply line, and who sets risk tier** — agent
   self-assessment (gameable), static checks, or user policy ("always ask
   about anything touching photos")?
5. **What's the candidate/restore-point retention policy** (storage cost vs
   the Time-Machine mental model of indefinite history)?
6. **What can never be rolled back** (emails sent, external API calls,
   payments) and how is that class marked at plan time, not discovered at
   rollback time?
7. **Stale candidates:** auto-expire, auto-restage against the moved tail,
   or accumulate for triage? (Restage invalidates approval — so auto-restage
   means re-review; that's correct but needs UX.)
8. **Two pending packages touching overlapping state:** promotion
   serialization exists in the spec's worldview (one at a time), but the
   queue/locking policy and its UI ("this change is waiting on that one")
   are undesigned.
9. **Uninstall/Disable:** platform-os-app-state.md's rules (no uninstall
   without verified inverse patch; no disable without a declared flag) were
   dropped in the Features cutover — resurrect, redesign, or explicitly
   abandon?
10. **Does promotion of *platform* computers (Wire, corpusd) use the same
    protocol** with a different approver (founder instead of owner), or a
    separate path? One protocol with authority parameters is the cleaner
    bet, but platform promotions interact with public state.

---

## 6. Sequencing

1. **Now (with the actor cutover):** P2 — enforce freshness CAS + approval
   gate; resurrect `owner_approved`; wire the Features app to the existing
   preview endpoint; reconcile platform-os-app-state.md with reality.
2. **Next:** P1 — promotion record + route-flip consumer + reconciler; spec
   conformance tests; restore-point timeline UI.
3. **Then:** P3 (Dolt branching at fork, merge at commit) and P4 (rollback
   window as state); contract-phase promotions as a separate class.
4. **With capsules (hybrid Milestones 1–3):** effect capture feeds the plan
   view; `guardForegroundSuperMutation` semantics retire in favor of
   capsule-by-default execution.
