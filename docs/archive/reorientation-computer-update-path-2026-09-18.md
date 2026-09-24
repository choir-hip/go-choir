# Reorientation: the owner-computer update path — 2026-09-18

Class: green (problem documentation + next-steps; no runtime change). Written
after the emergency restage of `computer-03335285269` onto the compose-fix
bundle. Owner is away; this is the triage surface for their return.

## The problem, restated

A retained owner computer is `snapshot_kind: constructed-computer-version`. Its
served release is pinned: `CHOIR_UPDATER_ROOT/current` → `releases/<digest>`,
and `current` only advances via `RestagePinnedRelease` / `ImportBaseline` /
self-dev apply. Two consequences:

1. **CI deploy does not update it.** `DEPLOY_ACTIVE_VM_REFRESH` preserves
   constructed realizations (`active_computers.status=empty`) — by design, a
   global deploy must not rewrite a constructed `ComputerVersion` in place.
   Documented in `docs/evidence/choir-rlm-restore-zero-constructed-refresh-skip-2026-09-09.md`.
2. **`computer refresh` does not update it.** Refresh reboots onto the canonical
   guest image but preserves `current`, so the pinned bundle persists.

Net: there is **no owner-facing "update my computer to the latest release"
command**. The only ways `current` moves are the heavyweight self-dev
materialization pipeline or a manual break-glass restage.

## What happened today (the "accident", explained)

- First deploy: the computer's `current` was invalid → `EnsureComputerSurface`
  on boot staged the canonical baseline → served `index-CoaQHCKe.js`
  (attachments). Looked like an update; was really "repair a broken `current`."
- Second deploy (compose fix): `current` was now valid → refresh preserved it →
  guest stayed on `CoaQHCKe` (attachments but not the fix).
- Fix applied: emergency restage — stop VM → mount `data.img` → rename
  `current`→`current.bak` → refresh → boot re-staged the canonical baseline →
  now serving `index-aVnZ-7Wy.js` + `MailApp-CWpTGVoJ.js` (compose fix).
  Runbook: `docs/runbooks/emergency-guest-restage.md`.

## The options for a durable fix

| Path | What it is | Status | Cost / risk |
| --- | --- | --- | --- |
| **A. Lightweight owner update command** | `choir computer update --release latest` → host stages canonical baseline into the computer's release store + repoints `current` (or drives `ImportBaseline` via the guest updater). | **Missing.** No such verb exists. | Low–medium. Reuses `ensureServingBaseline`/`ImportBaseline`; needs an owner-auth route + a "advance `current` to canonical" operation. This is the gap. |
| **B. Self-dev materialization** | `POST …/self-development/operations` → agent authors bundle → `accept_once`/`qualified_consensus` mode → approve → materializer builds release + `updater` applies + promotes route slot. | Implemented (`self_development_materializer.go`), gated by mode; `propose_only` is the current mode. | High for a frontend bump — drives the full agentic pipeline; intended for self-modification, not routine release updates. |
| **C. Emergency SSH restage** | Invalidate `current`; boot re-stages canonical baseline. | Works (used today). | Medium — manual, host SSH, mounts `data.img`, bypasses audit trail. Break-glass only. |

**Recommendation:** build **A** — a first-class, owner-authorized
`computer update` that advances `current` to the canonical baseline (or a named
release) through the existing updater, with a receipt. It is the missing product
surface; B is the long-term self-modification path and C is the stopgap.

## Open loops for triage (owner return)

1. **Owner update command (A).** Decide scope: "advance to canonical" only, or
   also "pin to release X"? Route: new vmctl/agentcore endpoint vs. extend
   `computer refresh` with an `--update-release` flag. Needs the receipt/audit
   story decided (does it append a computer event? a route transition?).
2. **Self-dev materialization readiness (B).** Mode is `propose_only`; the
   accept→materialize→rollback rehearsal (self-dev roadmap Mission 3) is
   unproven on staging. Is B the intended update path long-term, or only for
   self-modification? If the latter, A is required regardless.
3. **Constructed-computer deploy semantics.** Confirm the invariant "global
   deploy must not rewrite a constructed `ComputerVersion`" still holds, and
   that A is the sanctioned escape hatch (not weakening the CI skip).
4. **Attachments feature validation.** The compose fix is live on the owner
   computer; the deployed round-trip proof
   (`frontend/tests/mail-attachments-deployed.spec.js`) should be run against
   staging to close the mission's evidence.
5. **New injected ideas → reorientation.** Owner flagged new ideas needing a
   reorientation of next steps; this doc + `docs/ACTIVE.md` are the surface to
   re-plan against on return.

## References

- Runbook: `docs/runbooks/emergency-guest-restage.md`
- Problem evidence: `docs/evidence/choir-rlm-restore-zero-constructed-refresh-skip-2026-09-09.md`
- Self-dev roadmap: `docs/choir-self-development-roadmap-2026-08-11.md`
- Materializer: `internal/agentcore/self_development_materializer.go`
- Baseline staging: `internal/agentcore/rematerialize.go` (`EnsureComputerSurface`, `ensureServingBaseline`)
- Updater: `internal/updater/updater.go` (`VerifyCurrentRelease`, `ImportBaseline`, `RestagePinnedRelease`)
- Mission: `docs/mission-mail-attachments-and-app-cutover-2026-09-17.md`
