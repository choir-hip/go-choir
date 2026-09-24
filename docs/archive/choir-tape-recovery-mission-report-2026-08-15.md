# Choir Tape-Based Recovery — Mission Report

**Date:** 2026-08-15  
**Definition:** `docs/definitions/choir-tape-recovery-2026-08-13.md`  
**Status:** complete  
**Product on staging:** `4ac90583` (deployed 2026-08-14T23:24:20Z)  
**Docs land:** `ed932057` on `origin/main`  
**Independent review:** ACCEPT (checkpoint + rematerialization + serving join)

This is a completion report, not a Definition. Claims and receipts remain authoritative in the Definition and in `docs/evidence/tape-recovery-*.json`. Effects remain OFF.

---

## Verdict

A computer's restore set — the event-derived VM-local Dolt projection plus the computer-surface frontend — can be put back to a recorded head from the tape on staging.

That means:

1. A checkpoint that cannot be rebuilt from the tape is not recorded.
2. Restore reconstructs a new realization from the tape alone. It does not copy surviving local state and it does not swap only a release pointer.
3. Two computers serve two UIs. Unsigned callers get the host platform shell. Authenticated surface bytes are selected only after `vmctl` route resolve.
4. The owner path is `choir computer restore`. After restore, guest capability renews in-process. A subsequent `start` is not required.
5. On mismatch, the prior realization and its UI stay. Partial never greens.

The claim does **not** extend to the platform control-plane, auth material, shared cycle state, or non-restore-set guest files. It does not turn effects on. Owner-recovery checkpoint publication still carries a guest-attested witness trust split; that security review is post-mission by owner direction (2026-08-14).

---

## What this mission was for

Choir's durable-work kernel (settled 2026-07-21) and audited construction (settled 2026-07-17) proved that a computer can be built again from the same ComputerVersion: dispose `data.img`, rebuild identical code and artifacts. That is reconstruction of a *version*, not recovery of an *accumulated computer*.

On 2026-08-13 the owner made recoverability the priority. The frontend is part of the computer. World Wire stays tabled until after the actor RLM rewrite, so it inherits code-based orchestration rather than blocking restore.

The Definition's deliverable:

> A computer's restore set — the event-derived VM-local projection plus the computer-surface frontend — can be put back to a recorded head from the tape: rematerialized, verified against recorded content hashes, and the route/serving join flips visibility only on exact match.

Three identities stay distinct:

| Identity | Meaning |
|---|---|
| `restore_target_head` | The historical checkpoint being realized |
| `canonical_head` | The current forward chain, including the restore intent |
| `effective_content_witness` | The reversible projection that checkpoint commits to |

The event chain is the restore address. VM-local Dolt is a projection and audit witness, never an alternate head. Restore never erases history; returning to a prior point is a forward transaction.

---

## What was broken on 2026-08-13

The starting artifact, named in the Definition:

- Checkpoints bound event heads, an opaque state commitment, and release/reconstruction/materialization/verifier digests. They bound **no VM-local Dolt content witness** and **no frontend identity**.
- `ProjectionMaterializer` was non-runtime (an analysis CLI plus tests).
- `restorePrior` swapped only the release symlink. Nothing rematerialized VM-local Dolt state.
- The frontend was one host-global Caddy SPA (`frontend-current`) plus a per-computer API reverse-proxy. A computer's UI had no `ComputerID` and was not restored.

So a "restore" would have repainted restored state with today's CI SPA, and a "reconstructible" claim could not rebuild the projection the user actually operates.

Heresy discovered during the mission, in the order it blocked progress:

1. Checkpoints bind no VM-local content witness and no frontend identity.
2. Rematerialization has no runtime path; restore is release-pointer-only.
3. The frontend is host-global and outside the restore set.
4. Production never wires `CHOIR_UPDATER_ROOT` into Runtime, so checkpoint bind cannot see a staged guest SPA.
5. Rematerialize closes the guest store, so restore is not owner-reachable without restart.
6. A pre-rename `sandbox_id` SPA on a post-rename guest loops BIOS despite healthy `computer_id` bootstrap.

All six were repaired by completion. None were introduced by this mission.

---

## What was built

Five acceptance actions. All paid on staging.

### 1. Checkpoint witness

A published checkpoint now binds:

- target applied / effective event head
- CodeRef and ArtifactProgramRef
- a VM-local Dolt content witness (schema / table / content-root hashes; Dolt HEAD as audit receipt)
- a frontend identity derived from the release

Checkpoint creation fails closed while any behavior-bearing VM-local row is not event- or receipt-derivable, and while the served SPA is underivable.

First published owner-recovery checkpoint on the retained computer:

| Field | Value |
|---|---|
| Digest | `70f9ce2b…` |
| Computer | `computer-03335285269bdba4f94377e56879f9e6` |
| Epoch | 261, after RefreshVM onto `57e2992d` |
| Tables | 40 |
| Content root | `8701bed3…` |
| Frontend identity | `b95889ab…` (release-derived) |
| Verifier fields | empty (owner-recovery path) |

Later restore used checkpoint `67ab01f6…` (epoch 268, content root `9017bf72…`, same frontend identity).

### 2. Scope refusal

A user-computer restore that would touch the shared platform store or cycle state is refused. Those surfaces are not in the witness and not in the restore operand. Deployed as HTTP 400 on the retained computer; pinned by test.

Restore is scoped to the user computer by construction, not by convention.

### 3. Destructive tape-only rematerialization

`choir computer rematerialize-from-tape` (and restore, which uses the same reconstruction):

- quarantines the original realization (`data.img` and workspace)
- denies the restore implementation access to that original
- reconstructs into a sibling workspace from the canonical event tape and checkpoint-pinned content/artifact inputs
- verifies the reconstructed VM-local Dolt witness against the checkpoint
- restages the SPA onto the checkpoint-pinned release only after witness match
- refuses pin checkout as a completion route

First deployed pass (2026-08-14, `57e2992d`, checkpoint `70f9ce2b`): `witness_matched`, `original_denied`, `frontend_restaged`, `pin_checkout_used=false`. Quarantine: `/mnt/persistent/rematerialize-quarantine-20260814T221120…`. The guest was then degraded because rematerialize closed the store (`store_closed=true`) until RefreshVM epoch 262. That problem was named before it was patched.

### 4. Guest-static serving hop

The hop is **guest-static ComputerSurface after `vmctl` resolve**.

- Control-plane OUT: TLS, Caddy bootstrap, `/auth`, picker, proxy, vmctl, corpusd, NixOS.
- Computer surface IN: Desktop, Texture, apps, Settings, served asset graph.
- Capsule spawn / `StageGrantedRelease` fail closed without `frontend/index.html`.
- Proxy selects bytes only after route resolve. Missing SPA is HTTP 503.
- Unsigned callers receive the host platform shell, not a computer UI.

Live join on 2026-08-15T17:10Z, staging `4ac90583`:

| Surface | Script | `index.html` SHA-256 prefix | Notes |
|---|---|---|---|
| Unsigned `https://choir.news/` | `index-YTmyLpSn.js` | `4e2d1954` | Host shell; `X-Choir-Build-Service: proxy`; matches host `frontend-current` |
| Retained `computer-033352…` epoch 268 | `index-BH09hKq-.js` | `2c74a7b0` | Autoputer; matches restore SPA; bootstrap field `computer_id` |
| Secondary `computer-bb0f4fa…` epoch 12 | `index-BgRdleu6.js` | `1e62d8b9` | Autoputer; owner `a@b.com`; desktop loaded after hard reload |

Three hashes, three surfaces. `retained ≠ secondary ≠ platform shell`.

### 5. Owner-reachable whole-computer restore, with in-process renewal

Owner CLI, not SSH:

```
choir computer restore --computer computer-033352… --checkpoint-file …
```

Proof pattern: mutate VM-local Texture state and the served SPA, restore, then verify both the extractor witness and the served SPA bytes against the checkpoint. The live-only Texture document 404s. The mutated SPA marker is gone. SPA returns to `2c74a7b0`.

First owner restore (2026-08-14) paid the CLI path but left `store_closed=true`, so the guest was 502 until `choir computer start` epoch 264. That is credential re-exchange, not in-process renewal, so `capability_renewal_pass` stayed unpaid.

`4ac90583` reopens the rematerialized store in place (`Store.Reopen`). Epoch-268 restore on 2026-08-15T00:39Z returned `store_closed=false`. Process identity held. `choir computer replay-completeness` succeeded at 00:42:51Z (inside the 90s remaining window) and 00:45:06Z (after the original five-minute TTL) with no subsequent start. `capability_renewal_pass` paid.

---

## The actual trajectory

The product path was not a straight line. Each blocker was documented before repair.

### 2026-08-13 — define, then fail closed

Draft Definition `1ecbf9e6`. Local landings for witness bind (`6b28999e`), rematerialize (`d2558168`), guest-static hop (`490e779b`), SPA restage + owner restore (`cd403f98`), capsule freeze + capability grace (`8bbba401`), reconstruct-through-target + restore intent (`a2a80630`).

Deployed checkpoint bind on the retained computer returned **HTTP 409 served SPA is underivable** and did not publish. That is the correct fail-closed. Scope refusal was already HTTP 400.

Replay completeness on the retained computer was ineligible: nine behavior-bearing Texture tables were non-empty in live Dolt and not event-derivable. The computer had accumulated state the tape could not reproduce. Checkpoint must refuse, not silently drop rows.

`choir computer create` is unknown. `vmctl resolve` provisions only the primary desktop. The retained computer was the only interactive VM. The mission stayed on that computer rather than inventing a second one.

### 2026-08-14 — wire the updater, cut over workspace, publish

Host-staged `current/frontend` made guest `/` HTTP 200, but bind still 409: production never called `WithSelfDevelopmentUpdater`, so Runtime could not see the staged SPA. Named, then wired (`10dfa594`).

After the wire, bind still 409 — now on live-only rows, not an underivable SPA. Owner-ratified workspace-replace cutover flipped replay eligibility to true and bind to eligible. A new problem was then named: there is no owner-reachable checkpoint **publication** path for an accumulated computer, because verifier evidence sequencing is circular (you cannot produce verifier evidence of a checkpoint that does not yet exist).

Owner ratified option A (2026-08-14): `CheckpointRequest` gains `OwnerRecovery`. On that route verifier fields must be empty; platform verifies head / receipt / witness-shape server-side; guest attests the VM-local witness; restore reconstruction remains the enforcement that the witness is true. Route-projection and effects paths reject owner-recovery checkpoints (pinned by test). Security review deferred until missions complete.

`57e2992d` published checkpoint `70f9ce2b`, then rematerialize and owner restore paid their receipts with `store_closed=true` named as the remaining reachability gap.

### 2026-08-15 — reopen, renew, join

`4ac90583` reopens the store. Capability renewal paid without restart.

`serving_join` needed a second owner-reachable computer with a divergent UI. The owner authenticated as `a@b.com` on an existing secondary computer (`computer-bb0f4fa…`) rather than inventing `choir computer create`.

That computer looped CHOIR BIOS on "Waiting for computer identity" even though guest health was ready and `GET /api/shell/bootstrap` returned HTTP 200 with `computer_id` (204 bytes). Root cause: the served SPA was a pre-rename bundle (`Choir Secondary`, `index-C_MJakLv.js`) that read `bootstrapData.sandbox_id`. The API had renamed the field to `computer_id`. BIOS never saw identity.

Repair was owner-authorized SSH restage of a `computer_id` SPA onto that guest (genesis-baseline `5c73143f…`). Dolt was not touched. Rematerialize was not run. Prior release retained at `releases/secondary-divergent-20260815T030500Z`. Epoch 12, script `index-BgRdleu6.js`, hash `1e62d8b9`. Owner desktop loaded after hard reload.

Independent review held that the SSH restage does not unpaid the hop. `not_done_when` forbids restore reachable only through SSH; restore already used `choir computer restore`. The serving-join receipt does not claim `RestagePinnedRelease`.

Docs-only land `ed932057` stamped the Definition complete and promoted the effects Definition as the executable `/goal`.

---

## Required receipts

Landing required: `pushed_commit`, `ci`, `deploy`, `staging_build_identity`, plus the six product receipts below.

| Receipt | Paid | Proof |
|---|---|---|
| `checkpoint_witness` | 2026-08-14 | Published `70f9ce2b` on retained computer; 40-table witness; release-bound frontend identity |
| `scope_refusal` | 2026-08-13 | Deployed HTTP 400 for platform/cycle operands; bind fails closed on live-only rows |
| `destructive_rematerialization` | 2026-08-14 | `rematerialize-from-tape` through `70f9ce2b`; original quarantined; pin checkout unused |
| `owner_reachable_whole_computer_restore` | 2026-08-14 | `choir computer restore` after Texture + SPA mutation; SPA and Texture matched checkpoint |
| `capability_renewal_pass` | 2026-08-15 | Epoch 268 restore `store_closed=false`; replay-completeness at 00:42:51Z and 00:45:06Z; no start |
| `serving_join` | 2026-08-15 | Three distinct `index.html` hashes; two computers + unsigned shell; owner desktop on secondary |

CI for the product deploy: GitHub Actions `31848671245` success (Deploy to Staging Node B). Staging identity: `https://choir.news` at `4ac90583`.

---

## Independent review

Panel: `.agentic-consensus/tape-recovery-serving-join-20260815/`  
Receipt: `docs/evidence/tape-recovery-serving-join-independent-review-2026-08-15.md`

Eight routes attempted; five completed.

| Reviewer | Status | Verdict |
|---|---|---|
| OMP Gemini 3.6 Flash | ok | ACCEPT |
| OMP Cursor Grok 4.5 | ok | ACCEPT |
| OMP GPT-5.6 Sol | ok | ACCEPT |
| Cursor Agent | ok | ACCEPT |
| Devin | ok | REPAIR — docs/registry land only; `serving_join` paid; floor pass |
| Codex CLI | failed | `--autoputer` flag rejected |
| opencode | timed out | no verdict |
| OMP DeepSeek v4 Flash | timed out | no verdict |

Consensus that survived:

1. `serving_join` is paid. Live re-probe matched the receipt hashes.
2. SSH restage of the secondary SPA does not unpaid the hop.
3. Bundled floor (checkpoint + rematerialization + serving join) passes.
4. Do not stamp `complete` until the docs land. (Landed as `ed932057`.)
5. Owner-recovery security review is correctly deferred. Effects stay OFF.

The runner's overall exit 1 is a panelist-failure (three routes did not complete), not a product rejection.

---

## What this does not claim

- Effects. Decision-policy, qualified consensus, and effect promotion belong to `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`. Restore legs on that Definition are satisfied-by this mission and must not be independently greened.
- An owner restage-frontend verb. Secondary SPA restage was SSH/host script, documented as such, and is not restore.
- `choir computer create`. Still unknown. Do not invent it.
- Restore of platform control-plane, auth material, or cycle state.
- That the witness is sufficient for a state excursion never exercised.
- Production environment.
- World Wire end-to-end article/lineage production.

---

## What remains after this mission

**Executable `/goal`:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

That Definition owns policy-governed autonomy on top of this substrate: capsule-authored source change under effect-specific qualified consensus, E2 correction, acceptance-fenced restore through the paid restore set, and one exact irreversible email send under a stronger no-human-seat acceptance policy.

**Post-mission, not a tape-recovery gate:** security review of owner-recovery checkpoint publication (guest-attested VM-local witness; platform verifies shape, not content). Owner direction 2026-08-14: document it well, review once missions complete.

**Still true:**

- Do not rematerialize to reopen this mission.
- Do not invent `choir computer create`.
- Do not enable effects.

---

## Identities

| Role | ID |
|---|---|
| Retained computer | `computer-03335285269bdba4f94377e56879f9e6` |
| Retained VM | `candidate-fleet-e15cb89f25d963c220319b7b` |
| Secondary computer | `computer-bb0f4fa583c0cde14334818d946e6378` |
| Secondary VM | `candidate-fleet-49ee3bd0ec6f366a164c02d2` |
| Secondary owner | `a@b.com` / `0e5c45ab-44de-49cd-b07d-e58973b21ad5` |
| Staging | `https://choir.news` |
| Product commit | `4ac90583e389e3334efa57ce204d6df3235a68f1` |
| Complete stamp | `ed932057` |

### Load-bearing product commits

| Commit | Class | Change |
|---|---|---|
| `6b28999e` | red | Bind checkpoint witness; refuse platform/cycle restore |
| `d2558168` | red | Tape-only rematerialize product path |
| `490e779b` | red | Guest-static hop for computer-surface serving |
| `cd403f98` | red | Restage SPA on rematerialize; add owner restore |
| `8bbba401` | red | Fail-closed capsule freeze without frontend; grace-renew capability |
| `a2a80630` | red | Reconstruct through checkpoint head; append restore intent |
| `dda5fe6a` | red | Owner checkpoint bind fails closed on live-only rows |
| `10dfa594` | red | Wire guest Runtime to `CHOIR_UPDATER_ROOT` |
| `57e2992d` | red | Publish owner-recovery checkpoints without verifier evidence |
| `4ac90583` | red | Reopen rematerialized store in place |
| `ed932057` | green | Stamp complete; serving join paid |

---

## Evidence

| File | What it proves |
|---|---|
| `docs/evidence/tape-recovery-checkpoint-witness-published-2026-08-14.json` | First published restore-set checkpoint |
| `docs/evidence/tape-recovery-destructive-rematerialization-2026-08-14.json` | Tape-only reconstruct; original denied |
| `docs/evidence/tape-recovery-owner-restore-2026-08-14.json` | Owner CLI restore after state + SPA mutation |
| `docs/evidence/tape-recovery-capability-renewal-pass-2026-08-15.json` | Restore without restart; TTL crossed |
| `docs/evidence/tape-recovery-serving-join-2026-08-15.json` | Two computers, two UIs, plus host shell |
| `docs/evidence/tape-recovery-secondary-bootstrap-incident-2026-08-15.json` | `sandbox_id` BIOS loop; SSH restage |
| `docs/evidence/tape-recovery-serving-join-independent-review-2026-08-15.md` | Panel ACCEPT |
| `docs/evidence/tape-recovery-eligible-bind-no-owner-publication-path-2026-08-14.json` | Publication-path circularity named |
| `docs/memo-per-computer-frontend-2026-08-13.md` | Frontend-in-restore-set invariant |

Problem-documentation-first held: live-only rows, unwired updater root, closed store, and the secondary BIOS loop were written down before the corresponding repair.

---

## One-sentence architecture

The tape is the computer; the VM-local database and the served UI are projections of it; restore rebuilds those projections, checks the hashes, and only then lets anyone see them.
