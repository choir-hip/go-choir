# Effects computer-surface boot failure after image refresh — 2026-08-18

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Mutation class:** red — updater root and per-computer computer-surface serving.
**Protected surfaces:** updater root, computer-surface serving join, vmctl route resolution, deployment routing.

## Live observation

After the gateway timeout repair landed as `eb67848c740fbf3e3e8ef21bf2d78de7dedd9010`, staging reported the retained computer `computer-03335285269bdba4f94377e56879e6` active at realization epoch 302. Guest observability reported the same deployed guest build:

```text
commit eb67848c740fbf3e3e8ef21bf2d78de7dedd9010
started 2026-08-18T06:08:43Z
```

The named computer surface request then returned HTTP 503:

```text
self-development checkpoint: served SPA is underivable
```

The preceding product-path refresh evidence records that `choir.refresh_runtime=1` removes the persistent updater `current` pointer so the guest executes the freshly deployed immutable image. The same boot path does not restage an updater release or its `frontend/index.html` after removing that pointer.

## Source convergence

The serving path is explicit:

- `internal/autoputer/computer_surface.go` serves only `CHOIR_UPDATER_ROOT/current/frontend/index.html` and fails closed when it is absent.
- `nix/autoputer-vm.nix` sets `CHOIR_UPDATER_ROOT=/mnt/persistent/choir-updater`, and the refresh wrapper removes `current` when `choir.refresh_runtime=1`.
- The updater service starts before autoputer but only exposes baseline import through its permissioned Unix API; it does not import the immutable image at boot.
- `internal/agentcore/rematerialize.go` can derive and import the trusted `/nix/store/` baseline, but only when checkpoint binding or another self-development caller invokes `ensureCheckpointReleaseManifest`.
- `nix/node-b.nix` and `nix/node-a.nix` route the computer surface through the authenticated proxy/guest serving hop rather than the host-global `frontend-current` tree.

This leaves a fresh image boot with a healthy runtime API but no joined computer-surface release. A digest or route identity without served bytes is intentionally not green; the observed 503 is the correct fail-closed response, but boot has no product-path recovery to establish the baseline serving join.

## Belief delta

The refresh-runtime repair correctly prevents a stale persistent updater binary from masking the deployed guest image. It introduced or exposed a second boot invariant: after dropping `current`, the immutable image must be imported into the updater root before the computer surface can serve. The existing checkpoint-only fallback is too late for account boot and cannot be treated as a boot repair because checkpoint binding also performs replay eligibility and may publish owner-recovery evidence.

Leading root cause: refresh removes the only serving join and no boot-time, idempotent baseline-import path recreates it. This is source-confirmed; the product observation confirms the resulting fail-closed surface, while direct guest filesystem inspection is intentionally unavailable on the no-SSH product path.

## Safe repair boundary

Implement one narrow, idempotent boot/surface bootstrap that uses the existing trusted-baseline manifest and updater `ImportBaseline` contract, derives the exact current `ComputerVersion` route, and never appends an event, publishes a checkpoint, changes self-development mode, starts Super, or arms an outbox. Keep the computer surface fail-closed while route or baseline evidence is unavailable. Ordinary restart/recovery must continue to preserve a promoted `current` release.

The repair must have focused tests for: refresh with no current eventually serving the immutable baseline; existing current remaining untouched; route/baseline failure remaining 503; and no checkpoint/effect side effect. Rollback is a git revert of the repair commit; the retained computer remains propose-only with effects off.

## Remaining error and next safe probe

Do not call checkpoint, genesis, rematerialize, restore, or an operations POST to make the account surface appear. First land the documentation-only receipt, then implement and locally verify the boot/surface bootstrap. After the normal commit → CI → Node B deploy loop, owner-refresh the retained computer and verify the named surface returns the staged baseline with the expected build/serving identity before any self-development retry.

**Heresy delta:** discovered — refresh can leave a healthy guest with no served computer surface; introduced — none claimed until repair landing; repaired — none at this receipt.
**Conjecture delta:** the refresh/image identity repair remains supported; the new boot serving-join invariant is currently unproven.
**Rollback:** this receipt makes no runtime mutation; supersede or correct it in the Definition if a later product-path receipt disproves the source convergence.

## Post-landing observation

The boot bootstrap repair landed in workflow `32108952503` at `13a0ae7cebc7081753d0a93b92310b00ff41a6d0`. Staging host health and the activation receipt report commit `13a0ae7cebc7081753d0a93b92310b00ff41a6d0`; the autoputer package is installed at that commit. The deploy job intentionally did not refresh the retained computer because vmctl classified it as an immutable `constructed-computer-version` realization:

```text
active_computers: status=empty
computer-03335285269bdba4f94377e56879f9e6: epoch 302, immutable constructed realization
```

The retained guest therefore still reports the pre-repair autoputer build `eb67848c740fbf3e3e8ef21bf2d78de7dedd9010`, and the authenticated named surface still returns HTTP 503 with `self-development checkpoint: served SPA is underivable`. This is not deployed-identity proof for the repair; it is evidence that the normal landing loop preserved the immutable computer rather than exercising the new boot path.

The next safe probe is the Definition-authorized owner refresh of this same retained computer, not a second computer, checkpoint, rematerialization, restore, or self-development retry. The refresh must preserve the VM-local persistent state and then prove guest commit `13a0ae7cebc7081753d0a93b92310b00ff41a6d0` plus the named surface baseline before any further operation.

**Post-landing heresy delta:** discovered — successful host deployment can leave the retained immutable computer on the prior guest commit and therefore cannot prove a guest boot repair; introduced — none; repaired — none at this receipt.
**Post-landing conjecture delta:** the boot bootstrap implementation remains locally supported and deployed, but its behavior on the retained computer is unproven until the owner refresh crosses the immutable-realization boundary.
