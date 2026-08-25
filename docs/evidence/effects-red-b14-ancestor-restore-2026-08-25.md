# Effects Red: B14 Ancestor Restore of computer-0333528 — 2026-08-25

Computer: `computer-03335285269bdba4f94377e56879f9e6` (Family A supervised self-development)
Node: choiros-b, VM `candidate-fleet-e15cb89f25d963c220319b7b`
Mutation class: red (VM lifecycle, credential envelope, canonical tape). SSH used for
authorized recovery operations only; all product-path mutations (refresh API, envelope
mint endpoint) went through platform endpoints.

## Problem

After the failed cold-recover of 2026-08-25 (rec-1-e32d7c7f4c5fc7e8, phase stuck at
`swapped`), the computer's `data.img` was a **blank sparse staged image** and the guest
booted into the pre-genesis refusal loop
(`runtime api: submit internal run: computer is pre-genesis: run admission refused
(bootstrap-chain required; no canonical genesis on the tape)`). The platform head was
stable at 133,209 (`6e7424f0…`). The retained computer was unreachable and unfenced
from its own refusal loop.

## Heresies discovered (documented, not repaired)

1. **Cold-recover stages a blank image; B14 replay-only cannot bootstrap a blank disk.**
   `StageSparseImage` creates an empty `data.img`. The replay-only drive's
   `Reconstruct` on a tape-less, genesis-less image completes instantly with
   `seq=0 committed=0` (nothing local to materialize; the pre-genesis gate refuses a
   platform pull without local genesis). B14 replay-only requires an **ancestor image**
   (genesis + canonical tape + DEK) as the base. The prior successful B14
   (2026-08-24, to head 132,539) ran against the retained image for exactly this
   reason. Consensus panelist (grok, effects-durable-substrate review) predicted this;
   the operator missed it and paid for the blank-disk attempt.

2. **Cold-recover builds `credential.img` empty.** The maintenance boot's credential
   disk contained only `lost+found` (verified via `debugfs`). Any boot of an image
   with a tape crash-loops the autoputer:
   `acquire or recover computer event credential: lstat /run/choir-bootstrap/computer-event-envelope: no such file or directory`.

3. **vmctl-managed normal boots of the ancestor image hang silently (3/3).**
   Boots at 12:19:24, 12:54:50, and 13:43:40 (all vmctl-started, no
   `choir.runtime_*` params) produced firecracker startup lines and a single
   autoputer wrapper line, then zero console output and no network; the guest never
   reached the store phases. Boots carrying `choir.runtime_maintenance_hold=1`
   booted fully (4/4: 06:53 blank, 12:08, 13:12, 13:17, 14:09). Root cause is
   **unresolved**; the hang precedes the first autoputer log line. Consequence: the
   owner refresh cannot complete (it reboots the VM normally), so the platform route
   slot is unregistered (`choir.news` API returns `computer route not found`) and the
   computer is not product-reachable despite being live.

## Recovery performed (manual B14, working procedure)

1. Restored the ancestor image:
   `cp --reflink=always data.img.pre-upgrade-20260824T074931Z data.img`
   (blank staged image preserved at `data.img.blank-staged-rec-e32d7c7f`).
2. Replay drive boot: fc-config boot args + `choir.runtime_recovery_replay_only=1
   choir.runtime_maintenance_hold=1`; hand-minted credential envelope
   (`POST /internal/computers/credentials/issue` with the boot args' exact
   `realization_id`, canonical JSON, base64url, `mkfs.ext4 -d` into an 8 MiB
   `credential.img`, label CHOIR_CRED); tap up + host IP + INPUT/FORWARD accept +
   MASQUERADE + PREROUTING DNAT for service ports. Drive completed:
   `recovery replay-only drive complete (seq=0 committed=0); exiting without runtime
   start or reconciliation` — clean exit, no runtime, no reconciliation.
3. Serve boot: removed `replay_only`, kept `hold`; fresh envelope; runtime serving.
   Catch-up ran: injected texture worker updates loaded, retained runs re-driven,
   platform chain grew 133,209 → 133,318+ (the guest is appending as the event
   authority).

## Current state (2026-08-25 ~14:10Z)

- Guest: epoch-794 identity, runtime serving on :8085 (HTTP 200),
  `runtime: maintenance hold active (RUNTIME_MAINTENANCE_HOLD=1); refusing run
  admission + agent rewake while held` — **fenced**.
- Platform chain: growing past 133,318; computer appending.
- vmctl ownership: active, epoch 793→794 (pulse-reconciled), unheld host-side.
- Platform route: **unregistered** (refresh blocked by heresy 3).
- One residual startup error to watch:
  `runtime startup refused: actorruntime: reconcile Texture owner: reconcile subject
  5bd6de97…/computer-0333528…` (once, at 14:09:46, boot proceeded).

## Remaining work (in order)

1. Diagnose + repair heresy 3 (vmctl normal-boot hang) — it blocks refresh, route
   registration, and any unattended reboot of this computer. Suspect the runtime
   package apply path (the hold param may skip it); verify by diffing boot-time
   behavior with/without a pending package.
2. Complete a refresh → route registration → verify `choir.news` reachability.
3. Release the maintenance hold → computer fully live on the product path.
4. Then resume the effects mission: CoSuper authorship/freeze of candidate A
   (pre-declared foundation defect, five capsule-bound bundle refs).

## Evidence refs

- Console logs (node-b):
  `/var/lib/go-choir/vm-state/candidate-fleet-e15cb89f25d963c220319b7b/console-b14-{drive,serve,serve2,serve3}.log`
- Recovery journal: `rec-1-e32d7c7f4c5fc7e8` (phase `swapped`, failed cold-recover)
- Quarantines intact: `data.img.quarantine-1-{3c4bb0795b3356bd,5924af366f4ea289,e32d7c7f4c5fc7e8,e0b4a9d206a50f43}`,
  `data.img.pre-upgrade-20260824T074931Z`, `data.img.pre-hostdrive-20260824`,
  `data.img.pre-upgrade-20260825`, `data.img.pre-stop`
- Envelope mint endpoint: `POST /internal/computers/credentials/issue`
  (X-Internal-Caller), 201, TTL 10 min.

## Consensus amendment (2026-08-25, agentic-consensus panel, 10/12 succeeded)

The 4/4-hold vs 3/3-normal boot correlation is **confounded** (panel consensus;
locally verified in `nix/autoputer-vm.nix:135-158`):

- Every hung boot was a vmctl **refresh** boot carrying `choir.refresh_runtime=1`.
  The guest wrapper, on that param, **deletes `/mnt/persistent/choir-updater/current`**
  and falls back to the immutable **store binary** (the new a29f52cc autoputer).
- Every working manual boot omitted `refresh_runtime`, so the wrapper exec'd the
  **old dynamic binary** from the ancestor image's persistent
  `choir-updater/current`.
- `RUNTIME_MAINTENANCE_HOLD` is consulted only inside `agentcore.Runtime.Start`
  (`internal/agentcore/runtime.go:583`) — after store open and many log lines — so it
  cannot itself explain a hang before the first autoputer log.

Discriminated hypotheses for the hang, to be settled by a sequential boot matrix on a
reflink clone of the ancestor image (never the live disk):
(a) the new store binary hangs at first-open of the old persistent store state
    (silent migration/lock) when `current` is absent;
(b) the old dynamic binary boots fine only because it is old (baseline selection);
(c) a vmctl launch-path difference (tap/credential/waitForGuestReady).

Protective posture now: host hold SET (`held=true`,
"protect-live-guest-during-hang-diagnosis") so deploy-triggered active-VM refreshes
skip this computer; guest remains serving under its own env fence, appending.

Agreed plan skeleton (panel consensus): instrument wrapper/updater early markers, then
a sequential clone-boot matrix isolating refresh_runtime/binary-selection; repair the
demonstrated branch only; fail-closed fencing for heresies 1+2 (cold-recover refuses
without a validated ancestor base; prelaunch assertion that credential.img contains a
regular computer-event-envelope); land via CI/deploy; require THREE consecutive clean
vmctl-managed normal boots on the retained computer; unhold; one product-path owner
refresh; verify choir.news route + replay-completeness; resume self-development
(Conductor intake, no HTTP Super-start). The manual envelope/tap procedure stays
recovery-only and must not be scripted as steady-state.
