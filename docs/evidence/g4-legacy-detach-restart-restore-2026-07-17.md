# G4 Legacy Ownership Detach / Restart / Restore — Deployed Proof

Observed 2026-07-17 on staging Node B at deployed commit `b746fd71756fd9d0ed84a1317bda549cf351fd22`.

Mutation class: **red**. Protected surfaces: vmctl ownership persistence and route-authority serialization. No route transition, ComputerVersion construction, guest deletion, or fleet ownership mutation occurred. The only ownership mutation used the disposable synthetic legacy fixture `mission-restart-1778558340/primary`, and it was exactly restored.

## Deployment identity

- CI run [`29559210595`](https://github.com/choir-hip/go-choir/actions/runs/29559210595): success, including race shards, vet/build, differential SBOM, rolling-flake publication, and Node B deployment.
- `/var/lib/go-choir/deploy-receipt.json`: target commit `b746fd71756fd9d0ed84a1317bda549cf351fd22`, activated `2026-07-17T06:22:35Z`.
- `go-choir-vmctl.service`: active.
- Deployed vmctl contains `/internal/vmctl/computer-version-realizations/detach-legacy`.
- A `GET` to the new detach endpoint with the internal-caller header returned HTTP 405, proving the deployed handler and method refusal.

## Frozen fixture

- user/desktop: `mission-restart-1778558340/primary`;
- VM: `vm-bb1b05195c9186fa06f22455522e81ff`;
- route slot: `computer:mission-restart-1778558340:primary`;
- lifecycle: `hibernated`, epoch `1`;
- canonical inventory-row SHA-256: `683d9aaf581617b764a6f806b68f59acc4426c024438eb42fc4bc431db6789e1`;
- route resolve before detach: HTTP 404;
- no construction version or construction disk binding.

The detach authorization was signed with the owner-controlled Ed25519 promotion authority outside the repository. The signature binds the exact slot, VM, state, epoch, inventory digest, decision, key ID, and timestamp. No private material is present in the evidence packet.

## Executed lifecycle

1. Submitted the signed exact detach request.
2. Observed the exact ownership absent from vmctl list and a durable hash-addressed detach receipt in the existing ownership registry.
3. Confirmed the data image identity/geometry metadata was unchanged.
4. Restarted `go-choir-vmctl`; observed the ownership still detached, receipt still present, route absent, and image metadata unchanged.
5. Submitted the exact durable receipt to restore.
6. Observed the receipt consumed and the same ownership restored as `hibernated`, epoch `1`, with the same VM/user/desktop identity.
7. Restarted vmctl again; observed the restored ownership durable and image metadata unchanged.
8. Submitted a request whose inventory digest was changed without re-signing. vmctl returned HTTP 409 and the ownership-registry SHA-256 remained byte-identical.

The metadata invariant compared `device:inode:logical_bytes:allocated_blocks:mtime_epoch:ctime_epoch` for `data.img`. It was identical before detach, after detach, after detached restart, after restore, and after restored restart. No full-image scan or eager image copy was performed.

## Durable evidence

Machine-readable packet: [`g4-legacy-detach-restart-restore-2026-07-17.json`](g4-legacy-detach-restart-restore-2026-07-17.json)

Packet SHA-256: `57b1a26f866d36c1ce72f2d0a9af492d528014ce8778ae5b21f754251e7b0083`.

The packet embeds the signed request, detach receipt, restore request/response, forged-refusal response, final ownership, route-absence responses, deploy receipt, health response, all five image-stat observations, raw-artifact digests, and explicit boolean checks.

## Adjudication impact

The documented legacy dependency cycle is repaired on deployed staging for the tested exact lifecycle. This does **not** authorize fleet cutover. G4 still requires a frozen complete fleet inventory, per-route cutover and rollback plans, deterministic refusal checks, and independent gate adjudication before any fleet route CAS or durable legacy detach.

Heresy delta:

- discovered: none beyond the previously recorded legacy dependency cycle;
- introduced: none observed;
- repaired: deployed, signed, serialized, restart-durable legacy ownership detach/restore boundary for the exact tested lifecycle.

Rollback: revert `b746fd71` before any fleet detach. After a completed detach, use the durable exact restore receipt before reverting. The disposable fixture is already restored and requires no rollback action.
